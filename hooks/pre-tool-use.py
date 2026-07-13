#!/usr/bin/env python3
"""Rewrite shell tool calls from rm to graceful-rm when the host supports it."""

from __future__ import annotations

import json
import re
import sys

COMMAND_WORD = re.compile(
    r"(?P<prefix>^|[;&|]\s*|\n\s*)(?P<pre>(?:(?:sudo|command)\s+)*)rm(?=\s|$)"
)


def rewrite(command: str) -> str | None:
    """Replace rm only when it appears as a shell command word."""
    if "graceful-rm" in command:
        return None
    match = COMMAND_WORD.search(command)
    if not match:
        return None
    return (
        command[: match.start()]
        + match.group("prefix")
        + match.group("pre")
        + "graceful-rm"
        + command[match.end() :]
    )


def claude(payload: dict) -> int:
    if payload.get("tool_name") != "Bash":
        return 0
    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command")
    if not isinstance(command, str):
        return 0
    updated = rewrite(command)
    if updated is None:
        return 0
    tool_input = dict(tool_input)
    tool_input["command"] = updated
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "ask",
            "permissionDecisionReason": "Rewritten from rm to graceful-rm",
            "updatedInput": tool_input,
        }
    }))
    return 0


def codex(payload: dict) -> int:
    if payload.get("tool_name") != "Bash":
        return 0
    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command")
    if not isinstance(command, str):
        return 0
    updated = rewrite(command)
    if updated is None:
        return 0
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "allow",
            "updatedInput": {**tool_input, "command": updated},
        }
    }))
    return 0


def antigravity(payload: dict) -> int:
    call = payload.get("toolCall") or {}
    if call.get("name") != "run_command":
        print("{}")
        return 0
    args = call.get("args") or {}
    key = next((key for key in ("command", "cmd", "shellCommand") if isinstance(args.get(key), str)), None)
    if key is None:
        print("{}")
        return 0
    updated = rewrite(args[key])
    if updated is None:
        print("{}")
        return 0
    new_args = dict(args)
    new_args[key] = updated
    print(json.dumps({
        "decision": "deny",
        "reason": f"Replace rm with graceful-rm and retry: {new_args[key]}",
    }))
    return 0


def main() -> int:
    payload = json.load(sys.stdin)
    if "turn_id" in payload:
        return codex(payload)
    if "tool_name" in payload:
        return claude(payload)
    return antigravity(payload)


if __name__ == "__main__":
    raise SystemExit(main())
