# graceful-rm

`graceful-rm` is a safer Linux `rm` replacement implemented as a Go binary. It rejects high-risk system
targets, then moves accepted files and directories to
`~/.graceful-rm/trash` instead of deleting them immediately.

Trash entries expire after 72 hours. The installer uses a systemd timer on a
systemd host, `/etc/cron.d` in containers with cron, and command-time cleanup
when neither scheduler is available.

## Install

### Quick install

Install the system-wide command with one command:

```bash
curl -fsSL https://raw.githubusercontent.com/sealifes/graceful-rm/main/install.sh | bash -s -- --all
```

The command installs `graceful-rm` system-wide through `sudo`, then installs
all supported PreToolUse hooks in the current Git project. Use a specific hook
instead of `--all` when needed:

```bash
curl -fsSL https://raw.githubusercontent.com/sealifes/graceful-rm/main/install.sh | bash -s -- --codex
```

After installation, hooks can also be configured from the command itself:

```bash
graceful-rm --codex
graceful-rm --claude --global
graceful-rm --agy
graceful-rm --all
graceful-rm --alias
graceful-rm --uninstall-hook
graceful-rm --unalias
graceful-rm --uninstall
```

`--alias` detects `$SHELL` and adds `alias rm='graceful-rm'` to `~/.bashrc` or
`~/.zshrc`. It does not modify an existing `rm` alias/function; source the
reported rc file, or open a new shell, after installation. Hook installation
and alias installation are independent options.

Uninstall options are also safe and selective:

- `--uninstall-hook` removes graceful-rm hook entries from the current project while preserving other hooks.
- `--unalias` removes only the managed alias block from the detected rc file.
- `--uninstall` performs both operations.

Existing configuration files are backed up before hook removal.

The remote installer downloads the `main` archive before running, so piping it
does not depend on the shell's stdin being a local file. Set `GRACEFUL_RM_REF`
to install another branch or release ref explicitly.

### Install from a clone

Clone this repository and run the installer as root:

```bash
git clone https://github.com/sealifes/graceful-rm.git
cd graceful-rm
sudo ./scripts/install.sh
```

The command is then available system-wide:

```bash
graceful-rm --dry-run -- ./path/to/file
graceful-rm -- ./path/to/file
graceful-rm status
graceful-rm clean
graceful-rm restore <ID>
```

`status` lists the current user's trash entries, including their sizes and
original paths. `restore <ID>` restores one entry to its original path without
overwriting an existing file. `clean` clears the current user's trash
immediately after two confirmations requiring the exact word `yes`; run
`sudo graceful-rm clean` to clear all users' entries.

The installer does not replace `/bin/rm`, create an alias, or silently add
`sudo`.

The source installer builds `graceful-rm` and `graceful-rm-hook` with Go and
installs both as static system commands. Python and Node.js are not required.

## Safety rules

Blocked targets include `/` itself, system trees such as `/etc`, `/usr`, and
`/var`, symbolic links, mount points, the graceful-rm trash, and missing paths.
User data under `/home` and ordinary project paths are allowed. Cross-filesystem
targets are copied into the trash before their sources are removed. Review a
command with `--dry-run` first.

## Agent hooks

The bundled Codex `PreToolUse` hook rewrites shell command words such as
`rm -rf build` to `graceful-rm -rf build`. Claude Code and Antigravity adapter
examples are under `hooks/`. The hook only changes command words; text such as
`echo rm` is left alone.

For Codex, install the plugin through the marketplace or copy this repository
into the local plugin directory, then enable and trust its hooks. For the
other clients, copy the corresponding example into their hook configuration
and replace `${PLUGIN_ROOT}` with this repository's absolute path.

## Development checks

```bash
go test ./...
go vet ./...
bash -n scripts/install.sh
go run ./cmd/graceful-rm --dry-run -- README.md
```
