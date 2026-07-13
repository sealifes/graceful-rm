# graceful-rm

`graceful-rm` is a safer Linux `rm` replacement. It rejects high-risk system
targets, then moves accepted files and directories to
`/var/lib/graceful-rm/trash` instead of deleting them immediately.

Trash entries expire after 72 hours. The installer uses a systemd timer on a
systemd host, `/etc/cron.d` in containers with cron, and command-time cleanup
when neither scheduler is available.

## Install

### Quick install

Install the system-wide command with one command:

```bash
curl -fsSL https://raw.githubusercontent.com/sealifes/graceful-rm/main/install.sh | sudo bash
```

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
```

The installer does not replace `/bin/rm`, create an alias, or silently add
`sudo`.

## Safety rules

Blocked targets include `/` itself, system trees such as `/etc`, `/usr`, and
`/var`, symbolic links, mount points, the graceful-rm trash, missing paths, and
cross-filesystem moves. User data under `/home` and ordinary project paths are
allowed. Review a command with `--dry-run` first.

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
python3 -m py_compile scripts/graceful-rm hooks/pre-tool-use.py
bash -n scripts/install.sh
python3 scripts/graceful-rm --dry-run -- README.md
```
