package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var commandWord = regexp.MustCompile(`(?m)(^|[;&|][[:space:]]*|\n[[:space:]]*)((?:(?:sudo|command)[[:space:]]+)*)rm([[:space:]]|$)`)

func rewrite(command string) string {
	if strings.Contains(command, "graceful-rm") {
		return ""
	}
	loc := commandWord.FindStringSubmatchIndex(command)
	if loc == nil {
		return ""
	}
	start, end := loc[0], loc[1]
	return command[:start] + command[loc[2]:loc[3]] + command[loc[4]:loc[5]] + "graceful-rm" + command[loc[6]:loc[7]] + command[end:]
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--install" {
		if install(os.Args[2:]) != nil {
			os.Exit(1)
		}
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, ok := payload["turn_id"]; ok {
		codex(payload)
		return
	}
	if _, ok := payload["tool_name"]; ok {
		claude(payload)
		return
	}
	antigravity(payload)
}

func install(args []string) error {
	agent, global := "all", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent", "-a":
			if i+1 >= len(args) {
				return fmt.Errorf("missing agent")
			}
			agent = args[i+1]
			i++
		case "--global":
			global = true
		default:
			return fmt.Errorf("unknown option: %s", args[i])
		}
	}
	if agent != "codex" && agent != "claude" && agent != "antigravity" && agent != "all" {
		return fmt.Errorf("unsupported agent: %s", agent)
	}
	if global && agent != "claude" && agent != "all" {
		return fmt.Errorf("--global is only supported for claude")
	}
	project, err := projectRoot()
	if err != nil && !(global && agent == "claude") {
		return fmt.Errorf("run this command inside a Git project, or use --agent claude --global")
	}
	home, _ := os.UserHomeDir()
	type target struct{ path, kind string }
	var targets []target
	if agent == "codex" || agent == "all" {
		targets = append(targets, target{filepath.Join(project, ".codex", "hooks.json"), "codex"})
	}
	if agent == "claude" || agent == "all" {
		config := os.Getenv("CLAUDE_CONFIG_DIR")
		if config == "" {
			config = filepath.Join(home, ".claude")
		}
		if global {
			targets = append(targets, target{filepath.Join(config, "settings.json"), "claude"})
		} else {
			targets = append(targets, target{filepath.Join(project, ".claude", "settings.json"), "claude"})
		}
	}
	if agent == "antigravity" || agent == "all" {
		targets = append(targets, target{filepath.Join(project, ".agents", "hooks.json"), "antigravity"})
	}
	for _, t := range targets {
		if err := installTarget(t.path, t.kind); err != nil {
			return err
		}
		fmt.Println("installed", t.kind, "hook:", t.path)
	}
	return nil
}

func projectRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(out)), err
}
func installTarget(path, kind string) error {
	data := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &data); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		backup := path + ".backup-" + time.Now().Format("20060102150405")
		if err := os.WriteFile(backup, b, 0600); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	command := "graceful-rm-hook"
	if kind == "antigravity" {
		root := ensureMap(data, "graceful-rm")
		events := ensureSlice(root, "PreToolUse")
		if !hasHook(events) {
			events = append(events, map[string]any{"matcher": "run_command", "hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 10}}})
			root["PreToolUse"] = events
		}
		data["graceful-rm"] = root
	} else {
		root := ensureMap(data, "hooks")
		events := ensureSlice(root, "PreToolUse")
		if !hasHook(events) {
			matcher := "Bash"
			if kind == "codex" {
				matcher = "^Bash$"
			}
			events = append(events, map[string]any{"matcher": matcher, "hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 10}}})
			root["PreToolUse"] = events
		}
		data["hooks"] = root
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func ensureMap(parent map[string]any, key string) map[string]any {
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	parent[key] = m
	return m
}
func ensureSlice(parent map[string]any, key string) []any {
	if s, ok := parent[key].([]any); ok {
		return s
	}
	s := []any{}
	parent[key] = s
	return s
}
func hasHook(events []any) bool {
	for _, event := range events {
		if strings.Contains(fmt.Sprint(event), "graceful-rm") {
			return true
		}
	}
	return false
}

func commandInput(payload map[string]any) (map[string]any, string, bool) {
	if payload["tool_name"] != "Bash" {
		return nil, "", false
	}
	in, ok := payload["tool_input"].(map[string]any)
	if !ok {
		return nil, "", false
	}
	command, ok := in["command"].(string)
	return in, command, ok
}
func codex(payload map[string]any) {
	in, command, ok := commandInput(payload)
	if !ok {
		return
	}
	updated := rewrite(command)
	if updated == "" {
		return
	}
	in2 := map[string]any{}
	for k, v := range in {
		in2[k] = v
	}
	in2["command"] = updated
	output := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "allow", "updatedInput": in2}}
	_ = json.NewEncoder(os.Stdout).Encode(output)
}
func claude(payload map[string]any) {
	in, command, ok := commandInput(payload)
	if !ok {
		return
	}
	updated := rewrite(command)
	if updated == "" {
		return
	}
	in2 := map[string]any{}
	for k, v := range in {
		in2[k] = v
	}
	in2["command"] = updated
	output := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "ask", "permissionDecisionReason": "Rewritten from rm to graceful-rm", "updatedInput": in2}}
	_ = json.NewEncoder(os.Stdout).Encode(output)
}
func antigravity(payload map[string]any) {
	call, ok := payload["toolCall"].(map[string]any)
	if !ok || call["name"] != "run_command" {
		fmt.Println("{}")
		return
	}
	args, ok := call["args"].(map[string]any)
	if !ok {
		fmt.Println("{}")
		return
	}
	key := ""
	for _, k := range []string{"command", "cmd", "shellCommand"} {
		if _, ok := args[k].(string); ok {
			key = k
			break
		}
	}
	if key == "" {
		fmt.Println("{}")
		return
	}
	updated := rewrite(args[key].(string))
	if updated == "" {
		fmt.Println("{}")
		return
	}
	fmt.Printf("{\"decision\":\"deny\",\"reason\":\"Replace rm with graceful-rm and retry: %s\"}\n", strings.ReplaceAll(updated, "\"", "\\\""))
}
