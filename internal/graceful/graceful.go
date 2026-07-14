package graceful

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const retention = 3 * 24 * time.Hour

var protected = []string{"/", "/bin", "/boot", "/dev", "/etc", "/lib", "/lib64", "/opt", "/proc", "/root", "/run", "/sbin", "/srv", "/sys", "/usr", "/var"}

type metadata struct {
	OriginalPath string `json:"original_path"`
	DeletedAt    int64  `json:"deleted_at"`
	UID          int    `json:"uid"`
	GID          int    `json:"gid"`
}
type entry struct {
	path string
	meta metadata
}

func TrashRoot() string {
	if v := os.Getenv("GRACEFUL_RM_TRASH"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".graceful-rm", "trash")
}

func reject(path string) (string, string, error) {
	if path == "" || path == "." || path == ".." {
		return "", "empty, current-directory, and parent-directory targets are blocked", nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "target does not exist", nil
		}
		return "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", "symbolic links are blocked; remove the link explicitly after inspection", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", "", err
	}
	trash, _ := filepath.Abs(TrashRoot())
	if resolved == trash || resolved == filepath.Dir(trash) || isBelow(resolved, trash) {
		return "", "the graceful-rm trash is protected", nil
	}
	for _, root := range protected {
		if resolved == root || (root != "/" && isBelow(resolved, root)) {
			return "", "protected system path: " + resolved, nil
		}
	}
	return resolved, "", nil
}

func isBelow(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func Move(raw string, dryRun bool) error {
	path, reason, err := reject(raw)
	if err != nil {
		return fmt.Errorf("cannot inspect %s: %w", raw, err)
	}
	if reason != "" {
		return fmt.Errorf("refusing %s: %s", raw, reason)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dest := filepath.Join(TrashRoot(), fmt.Sprint(currentUID()), fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().UnixNano()))
	if dryRun {
		fmt.Printf("would move %s -> %s\n", path, dest)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	if err := os.Rename(path, dest); err == nil {
		if e := writeMeta(dest, path); e != nil {
			return fmt.Errorf("moved %s, but metadata write failed: %w", path, e)
		}
		fmt.Printf("moved %s -> %s\n", path, dest)
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("cannot move %s: %w", raw, err)
	}
	if err := copyPath(path, dest); err != nil {
		_ = removePath(dest)
		return fmt.Errorf("cross-filesystem copy failed; source was not changed: %w", err)
	}
	before, _ := os.Stat(path)
	if before == nil || !os.SameFile(before, info) || before.Mode() != info.Mode() {
		_ = removePath(dest)
		return errors.New("source changed while it was being copied")
	}
	if err := writeMeta(dest, path); err != nil {
		_ = removePath(dest)
		return fmt.Errorf("copied to trash but source was kept: %w", err)
	}
	if err := removePath(path); err != nil {
		return fmt.Errorf("copied to trash but kept source because final removal was not safe: %w", err)
	}
	fmt.Printf("moved %s -> %s\n", path, dest)
	return nil
}

func metaPath(path string) string { return path + ".json" }
func writeMeta(path, original string) error {
	m := metadata{OriginalPath: original, DeletedAt: time.Now().Unix(), UID: currentUID(), GID: currentGID()}
	b, _ := json.Marshal(m)
	return os.WriteFile(metaPath(path), append(b, '\n'), 0600)
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, e := os.Readlink(src)
		if e != nil {
			return e
		}
		return os.Symlink(target, dst)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	return closeErr
}
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.Mkdir(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if path == src {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.Type()&os.ModeSymlink != 0 {
			link, er := os.Readlink(path)
			if er != nil {
				return er
			}
			return os.Symlink(link, target)
		}
		if d.IsDir() {
			return os.Mkdir(target, d.Type().Perm())
		}
		return copyPath(path, target)
	})
}
func removePath(path string) error { return os.RemoveAll(path) }

func Cleanup() int {
	cutoff := time.Now().Add(-retention)
	for _, ownerDir := range ownerDirs() {
		children, _ := os.ReadDir(ownerDir)
		for _, child := range children {
			p := filepath.Join(ownerDir, child.Name())
			info, err := os.Lstat(p)
			if err == nil && !info.ModTime().After(cutoff) {
				_ = removePath(p)
			}
		}
	}
	return 0
}

func ownerDirs() []string {
	root := TrashRoot()
	if currentUID() != 0 {
		p := filepath.Join(root, fmt.Sprint(currentUID()))
		if info, _ := os.Stat(p); info != nil && info.IsDir() {
			return []string{p}
		}
		return nil
	}
	dirs, _ := os.ReadDir(root)
	var out []string
	for _, d := range dirs {
		if d.IsDir() {
			out = append(out, filepath.Join(root, d.Name()))
		}
	}
	return out
}
func readMeta(path string) metadata {
	b, err := os.ReadFile(metaPath(path))
	if err != nil {
		return metadata{}
	}
	var m metadata
	_ = json.Unmarshal(b, &m)
	return m
}
func size(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
func formatSize(n int64) string {
	v := float64(n)
	for _, u := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
		if v < 1024 || u == "TiB" {
			if u == "B" {
				return fmt.Sprintf("%d B", n)
			}
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= 1024
	}
	return fmt.Sprintf("%d B", n)
}

func list() []entry {
	var out []entry
	for _, dir := range ownerDirs() {
		children, _ := os.ReadDir(dir)
		for _, child := range children {
			if strings.HasSuffix(child.Name(), ".json") {
				continue
			}
			p := filepath.Join(dir, child.Name())
			out = append(out, entry{path: p, meta: readMeta(p)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return filepath.Base(out[i].path) < filepath.Base(out[j].path) })
	return out
}
func Status() int {
	rows := list()
	fmt.Println("Trash box:", TrashRoot())
	if len(rows) == 0 {
		fmt.Println("Trash is empty.")
		return 0
	}
	idw, sw := 2, 4
	for _, e := range rows {
		if len(filepath.Base(e.path)) > idw {
			idw = len(filepath.Base(e.path))
		}
		if len(formatSize(size(e.path))) > sw {
			sw = len(formatSize(size(e.path)))
		}
	}
	fmt.Printf("%-*s  %*s  %-19s  %s\n", idw, "ID", sw, "SIZE", "DELETED AT", "ORIGINAL PATH")
	for _, e := range rows {
		deleted := "unknown"
		if e.meta.DeletedAt != 0 {
			deleted = time.Unix(e.meta.DeletedAt, 0).Local().Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%-*s  %*s  %-19s  %s\n", idw, filepath.Base(e.path), sw, formatSize(size(e.path)), deleted, e.meta.OriginalPath)
	}
	return 0
}

func Clean(in io.Reader, out io.Writer) int {
	rows := list()
	if len(rows) == 0 {
		fmt.Fprintln(out, "Trash is empty.")
		return 0
	}
	var total int64
	for _, e := range rows {
		total += size(e.path)
	}
	fmt.Fprintf(out, "This will permanently delete %d trash entr%s (%s).\n", len(rows), map[bool]string{true: "y", false: "ies"}[len(rows) == 1], formatSize(total))
	reader := bufio.NewReader(in)
	for i, prompt := range []string{"First confirmation - type 'yes' to continue: ", "Second confirmation - type 'yes' again to continue: "} {
		fmt.Fprint(out, prompt)
		s, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(out, "\nClean cancelled.")
			return 1
		}
		if strings.TrimSpace(s) != "yes" {
			fmt.Fprintln(out, "Clean cancelled.")
			return 1
		}
		if i == 1 {
			break
		}
	}
	for _, dir := range ownerDirs() {
		children, _ := os.ReadDir(dir)
		for _, c := range children {
			_ = removePath(filepath.Join(dir, c.Name()))
		}
	}
	fmt.Fprintln(out, "Trash cleared.")
	return 0
}

func Restore(id string) int {
	var found string
	for _, dir := range ownerDirs() {
		p := filepath.Join(dir, id)
		if _, err := os.Lstat(p); err == nil {
			if found != "" {
				fmt.Fprintln(os.Stderr, "graceful-rm: trash entry ID is ambiguous:", id)
				return 1
			}
			found = p
		}
	}
	if found == "" {
		fmt.Fprintln(os.Stderr, "graceful-rm: trash entry not found:", id)
		return 1
	}
	m := readMeta(found)
	if m.OriginalPath == "" {
		fmt.Fprintln(os.Stderr, "graceful-rm: metadata missing original path for", id)
		return 1
	}
	if _, err := os.Lstat(m.OriginalPath); err == nil {
		fmt.Fprintln(os.Stderr, "graceful-rm: refusing to overwrite existing path:", m.OriginalPath)
		return 1
	}
	parent := filepath.Dir(m.OriginalPath)
	pi, err := os.Stat(parent)
	if err != nil || !pi.IsDir() {
		fmt.Fprintln(os.Stderr, "graceful-rm: restore parent directory does not exist:", parent)
		return 1
	}
	if err = os.Rename(found, m.OriginalPath); err != nil {
		if !errors.Is(err, syscall.EXDEV) {
			fmt.Fprintln(os.Stderr, "graceful-rm: cannot restore:", err)
			return 1
		}
		if err = copyPath(found, m.OriginalPath); err != nil {
			fmt.Fprintln(os.Stderr, "graceful-rm: restore failed; trash entry was kept:", err)
			return 1
		}
		_ = removePath(found)
	}
	_ = os.Remove(metaPath(found))
	fmt.Println("restored", m.OriginalPath)
	return 0
}
