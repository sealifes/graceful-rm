---
name: graceful-rm
description: Install, inspect, and use the safer system-wide graceful-rm command.
---

# Graceful RM

Use this plugin to install and operate the Go-based `graceful-rm` command on Linux or Windows.

## Safety contract

- `graceful-rm` moves accepted paths to `~/.graceful-rm/trash`.
- It rejects protected system paths, symbolic links, mount points, the trash itself, and missing paths.
- Cross-filesystem targets are copied into the trash and the source is removed only after identity checks.
- The session-start hook is read-only and must never use `sudo` automatically.
- Installation requires explicit root authorization because it writes `/usr/local/bin` and the host scheduler configuration.

## Install

From this plugin directory, run:

```bash
sudo ./scripts/install.sh
```

The installer uses a systemd timer only when systemd is actually running as PID 1. In containers it uses `/etc/cron.d` when available; otherwise cleanup is also attempted when the command is used.

Then verify:

```bash
graceful-rm --dry-run -- ./path
graceful-rm --alias
systemctl status graceful-rm-cleanup.timer  # systemd hosts only
```

`--alias` detects the user's Bash or Zsh from `$SHELL` and adds a guarded
`rm` alias to the matching rc file. It does not overwrite an existing alias or
function; source the reported rc file after installation.

Use `graceful-rm --uninstall-hook` to remove only graceful-rm hook entries,
`graceful-rm --unalias` to remove only the managed shell alias, or
`graceful-rm --uninstall` for both. Hook configuration files are backed up
before managed entries are removed.

Never replace `/bin/rm` or create an alias automatically. Users can opt into an alias after testing.

## Pre-tool integration

The intended agent integration is a `PreToolUse` hook. The Go `graceful-rm-hook` binary reads host hook JSON and rewrites shell calls such as `rm -rf build` to `graceful-rm -rf build`. It is conservative: it does not rewrite text such as `echo rm`, and it never adds `sudo`.

Codex discovers the bundled `hooks/hooks.json` automatically after the plugin is enabled; review and trust it with `/hooks`. Codex supports `updatedInput` with `permissionDecision: "allow"` for Bash hooks. Claude Code supports the equivalent `updatedInput`; copy `hooks/claude-code-settings.json` into the appropriate settings file and replace its placeholder path.

Antigravity supports `PreToolUse` hooks in `hooks.json`, but its documented JSON hook output exposes allow/deny/ask rather than a documented updated-input field. Its adapter therefore denies the original call and tells the agent to retry with the rewritten command; it does not pretend to perform an undocumented mutation.
