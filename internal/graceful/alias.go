package graceful

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const aliasMarker = "# graceful-rm alias"

func InstallAlias() int {
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "" {
		if out, err := exec.Command("ps", "-p", fmt.Sprint(os.Getppid()), "-o", "comm=").Output(); err == nil {
			shell = filepath.Base(strings.TrimSpace(string(out)))
		}
	}
	var rc string
	switch shell {
	case "bash":
		rc = filepath.Join(os.Getenv("HOME"), ".bashrc")
	case "zsh":
		rc = filepath.Join(os.Getenv("HOME"), ".zshrc")
	default:
		fmt.Fprintf(os.Stderr, "graceful-rm: unsupported shell %q; --alias supports bash and zsh\n", shell)
		return 1
	}

	content, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "graceful-rm: cannot read %s: %v\n", rc, err)
		return 1
	}
	text := string(content)
	if strings.Contains(text, aliasMarker) {
		fmt.Printf("graceful-rm: alias already installed in %s\n", rc)
		return 0
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "alias rm=") || strings.HasPrefix(trimmed, "function rm") || trimmed == "rm()" {
			fmt.Fprintf(os.Stderr, "graceful-rm: an existing rm alias or function was found in %s; no changes made\n", rc)
			return 1
		}
	}
	block := "\n" + aliasMarker + "\nalias rm='graceful-rm'\n"
	if err := os.WriteFile(rc, append(content, []byte(block)...), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "graceful-rm: cannot update %s: %v\n", rc, err)
		return 1
	}
	fmt.Printf("installed graceful-rm alias in %s\n", rc)
	fmt.Printf("run: source %s\n", rc)
	return 0
}

func UninstallAlias() int {
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "" {
		if out, err := exec.Command("ps", "-p", fmt.Sprint(os.Getppid()), "-o", "comm=").Output(); err == nil {
			shell = filepath.Base(strings.TrimSpace(string(out)))
		}
	}
	var rc string
	switch shell {
	case "bash":
		rc = filepath.Join(os.Getenv("HOME"), ".bashrc")
	case "zsh":
		rc = filepath.Join(os.Getenv("HOME"), ".zshrc")
	default:
		fmt.Fprintf(os.Stderr, "graceful-rm: unsupported shell %q; --unalias supports bash and zsh\n", shell)
		return 1
	}
	content, err := os.ReadFile(rc)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("graceful-rm: no alias found in %s\n", rc)
			return 0
		}
		fmt.Fprintf(os.Stderr, "graceful-rm: cannot read %s: %v\n", rc, err)
		return 1
	}
	text := string(content)
	block := "\n" + aliasMarker + "\nalias rm='graceful-rm'\n"
	if !strings.Contains(text, aliasMarker) {
		fmt.Printf("graceful-rm: no managed alias found in %s\n", rc)
		return 0
	}
	updated := strings.Replace(text, block, "", 1)
	if updated == text {
		fmt.Fprintf(os.Stderr, "graceful-rm: managed alias block in %s has unexpected contents; no changes made\n", rc)
		return 1
	}
	if err := os.WriteFile(rc, []byte(updated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "graceful-rm: cannot update %s: %v\n", rc, err)
		return 1
	}
	fmt.Printf("removed graceful-rm alias from %s\n", rc)
	fmt.Printf("run: source %s\n", rc)
	return 0
}
