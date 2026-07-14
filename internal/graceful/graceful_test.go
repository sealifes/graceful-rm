package graceful

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectAllowsRemovingFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(target, []byte("target\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Fatal(err)
	}

	resolved, reason, err := reject(link)
	if err != nil {
		t.Fatal(err)
	}
	if reason != "" {
		t.Fatalf("reject() refused final symlink: %s", reason)
	}
	if resolved != link {
		t.Fatalf("resolved path = %q, want %q", resolved, link)
	}
}

func TestRejectProtectsPathThroughSymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "etc-link")
	if err := os.Symlink("/etc", parent); err != nil {
		t.Fatal(err)
	}

	_, reason, err := reject(filepath.Join(parent, "passwd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "protected system path") {
		t.Fatalf("reason = %q, want protected-system-path refusal", reason)
	}
}
