#!/usr/bin/env bash
set -euo pipefail

agent=all
global_claude=false
uninstall=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    -a|--agent) agent="${2:?missing agent name}"; shift 2 ;;
    --global) global_claude=true; shift ;;
    --uninstall) uninstall=true; shift ;;
    -h|--help) echo "Usage: $0 [--agent codex|claude|antigravity|all] [--global]"; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

hook_bin="$(command -v graceful-rm-hook || true)"
if [[ -z "$hook_bin" ]]; then
  echo "graceful-rm-hook is not installed; run the main installer first" >&2
  exit 1
fi
args=(--install --agent "$agent")
if [[ "$uninstall" == true ]]; then args=(--uninstall --agent "$agent"); fi
if [[ "$global_claude" == true ]]; then args+=(--global); fi
exec "$hook_bin" "${args[@]}"
