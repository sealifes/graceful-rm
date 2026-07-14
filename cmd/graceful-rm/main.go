package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sealifes/graceful-rm/internal/graceful"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	args = expandShortFlags(args)
	if len(args) == 1 {
		switch args[0] {
		case "status":
			return graceful.Status()
		case "clean":
			return graceful.Clean(os.Stdin, os.Stdout)
		}
	}
	if len(args) == 2 && args[0] == "restore" {
		return graceful.Restore(args[1])
	}

	fs := flag.NewFlagSet("graceful-rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: graceful-rm [OPTIONS] PATH...")
		fmt.Fprintln(fs.Output(), "       graceful-rm status | clean | restore ID")
		fmt.Fprintln(fs.Output(), "")
		fs.PrintDefaults()
		fmt.Fprintln(fs.Output(), "\nCommands:")
		fmt.Fprintln(fs.Output(), "  graceful-rm status       list trash entries and file sizes")
		fmt.Fprintln(fs.Output(), "  graceful-rm clean        clear the current user's trash (two confirmations)")
		fmt.Fprintln(fs.Output(), "  graceful-rm restore ID   restore one trash entry")
	}
	dryRun := fs.Bool("dry-run", false, "show moves without changing files")
	cleanup := fs.Bool("cleanup", false, "remove trash entries older than three days")
	codex := fs.Bool("codex", false, "install the Codex PreToolUse hook")
	claude := fs.Bool("claude", false, "install the Claude Code PreToolUse hook")
	agy := fs.Bool("agy", false, "install the Antigravity PreToolUse hook")
	antigravity := fs.Bool("antigravity", false, "alias for --agy")
	all := fs.Bool("all", false, "install all supported PreToolUse hooks")
	global := fs.Bool("global", false, "use global Claude Code settings")
	fs.Bool("r", false, "accepted for rm compatibility")
	fs.Bool("R", false, "accepted for rm compatibility")
	fs.Bool("recursive", false, "accepted for rm compatibility")
	fs.Bool("f", false, "accepted for rm compatibility")
	fs.Bool("force", false, "accepted for rm compatibility")
	fs.Bool("i", false, "accepted for rm compatibility")
	fs.Bool("interactive", false, "accepted for rm compatibility")
	fs.Bool("v", false, "accepted for rm compatibility")
	fs.Bool("verbose", false, "accepted for rm compatibility")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	hookCount := 0
	for _, enabled := range []bool{*codex, *claude, *agy || *antigravity, *all} {
		if enabled {
			hookCount++
		}
	}
	if hookCount > 0 {
		if hookCount != 1 || len(fs.Args()) > 0 || (*global && !*claude) {
			fmt.Fprintln(os.Stderr, "graceful-rm: hook options cannot be combined with paths; --global is only valid with --claude")
			return 2
		}
		agent := "all"
		if *codex {
			agent = "codex"
		}
		if *claude {
			agent = "claude"
		}
		if *agy || *antigravity {
			agent = "antigravity"
		}
		cmdArgs := []string{"--agent", agent}
		if *global {
			cmdArgs = append(cmdArgs, "--global")
		}
		cmd := exec.Command("/usr/local/share/graceful-rm/scripts/install-hooks.sh", cmdArgs...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode()
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *cleanup {
		return graceful.Cleanup()
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "graceful-rm: at least one path is required")
		fs.PrintDefaults()
		return 2
	}
	code := 0
	for _, path := range paths {
		if err := graceful.Move(path, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "graceful-rm: %v\n", err)
			code = 1
		}
	}
	return code
}

func expandShortFlags(args []string) []string {
	known := map[byte]bool{'r': true, 'R': true, 'f': true, 'i': true, 'v': true}
	var expanded []string
	for _, arg := range args {
		if len(arg) > 2 && strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			ok := true
			for i := 1; i < len(arg); i++ {
				if !known[arg[i]] {
					ok = false
					break
				}
			}
			if ok {
				for i := 1; i < len(arg); i++ {
					expanded = append(expanded, "-"+string(arg[i]))
				}
				continue
			}
		}
		expanded = append(expanded, arg)
	}
	return expanded
}
