#!/usr/bin/env bash
set -euo pipefail

agent=all
global_claude=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    -a|--agent) agent="${2:?missing agent name}"; shift 2 ;;
    --global) global_claude=true; shift ;;
    -h|--help) echo "Usage: $0 [--agent codex|claude|antigravity|all] [--global]"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

case ",$agent," in
  *,codex,*|*,claude,*|*,antigravity,*|*,all,*) ;;
  *) echo "unsupported agent: $agent" >&2; exit 2 ;;
esac
if [[ "$global_claude" == true && "$agent" != claude && "$agent" != all ]]; then
  echo "--global is only supported for claude" >&2
  exit 2
fi

project_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "$project_root" && "$global_claude" != true ]]; then
  echo "run this command inside a Git project, or use --agent claude --global" >&2
  exit 1
fi

hook_dir="${XDG_DATA_HOME:-$HOME/.local/share}/graceful-rm"
mkdir -p "$hook_dir"
install -m 0755 "$(dirname "$0")/../hooks/pre-tool-use.py" "$hook_dir/pre-tool-use.py"

export GRACEFUL_RM_AGENT="$agent"
export GRACEFUL_RM_GLOBAL_CLAUDE="$global_claude"
export GRACEFUL_RM_HOOK="$hook_dir/pre-tool-use.py"
export GRACEFUL_RM_PROJECT_ROOT="$project_root"
python3 - <<'PY'
import json
import os
import shutil
import time
from pathlib import Path

agent = os.environ["GRACEFUL_RM_AGENT"]
hook = os.environ["GRACEFUL_RM_HOOK"]
project = Path(os.environ["GRACEFUL_RM_PROJECT_ROOT"])
global_claude = os.environ["GRACEFUL_RM_GLOBAL_CLAUDE"] == "true"
targets = []
if agent in ("codex", "all"):
    targets.append((project / ".codex" / "hooks.json", "codex"))
if agent in ("claude", "all"):
    config = Path(os.environ.get("CLAUDE_CONFIG_DIR", "~/.claude")).expanduser()
    targets.append(((config if global_claude else project / ".claude") / "settings.json", "claude"))
if agent in ("antigravity", "all"):
    targets.append((project / ".agents" / "hooks.json", "antigravity"))

for path, kind in targets:
    path.parent.mkdir(parents=True, exist_ok=True)
    data = {}
    if path.exists():
        backup = path.with_name(path.name + ".backup-" + time.strftime("%Y%m%d%H%M%S"))
        shutil.copy2(path, backup)
        data = json.loads(path.read_text())
    command = f"python3 {hook}"
    if kind in ("codex", "claude"):
        events = data.setdefault("hooks", {}).setdefault("PreToolUse", [])
        if not any("graceful-rm" in json.dumps(item) for item in events):
            events.append({"matcher": "^Bash$" if kind == "codex" else "Bash", "hooks": [{"type": "command", "command": command, "timeout": 10}]})
    else:
        events = data.setdefault("graceful-rm", {}).setdefault("PreToolUse", [])
        if not any("graceful-rm" in json.dumps(item) for item in events):
            events.append({"matcher": "run_command", "hooks": [{"type": "command", "command": command, "timeout": 10}]})
    path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"installed {kind} hook: {path}")
PY
