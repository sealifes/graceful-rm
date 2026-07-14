#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "install.sh must run as root" >&2
  exit 1
fi

source_dir="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT
go_cmd="$(command -v go || true)"
if [[ -z "$go_cmd" ]]; then
  for candidate in /usr/local/go/bin/go /usr/lib/go/bin/go; do
    if [[ -x "$candidate" ]]; then go_cmd="$candidate"; break; fi
  done
fi
if [[ -x "$source_dir/bin/graceful-rm" && -x "$source_dir/bin/graceful-rm-hook" ]]; then
  graceful_rm_bin="$source_dir/bin/graceful-rm"
  hook_bin="$source_dir/bin/graceful-rm-hook"
elif [[ -n "$go_cmd" ]]; then
  "$go_cmd" build -trimpath -ldflags='-s -w' -o "$build_dir/graceful-rm" "$source_dir/cmd/graceful-rm"
  "$go_cmd" build -trimpath -ldflags='-s -w' -o "$build_dir/graceful-rm-hook" "$source_dir/cmd/graceful-rm-hook"
  graceful_rm_bin="$build_dir/graceful-rm"
  hook_bin="$build_dir/graceful-rm-hook"
else
  echo "install.sh: Go is required when installing from source" >&2
  exit 1
fi
install -D -m 0755 "$graceful_rm_bin" /usr/local/bin/graceful-rm
install -D -m 0755 "$hook_bin" /usr/local/bin/graceful-rm-hook
install -D -m 0755 "$(dirname "$0")/install-hooks.sh" /usr/local/share/graceful-rm/scripts/install-hooks.sh
install -d -o root -g root -m 0755 /var/lib/graceful-rm
install -d -o root -g root -m 01777 /var/lib/graceful-rm/trash

if [[ "$(ps -p 1 -o comm= 2>/dev/null || true)" == "systemd" ]] && command -v systemctl >/dev/null 2>&1; then
  install -D -m 0644 "$(dirname "$0")/graceful-rm-cleanup.service" /etc/systemd/system/graceful-rm-cleanup.service
  install -D -m 0644 "$(dirname "$0")/graceful-rm-cleanup.timer" /etc/systemd/system/graceful-rm-cleanup.timer
  systemctl daemon-reload
  systemctl enable --now graceful-rm-cleanup.timer
  echo "Installed /usr/local/bin/graceful-rm and enabled the 3-day systemd cleanup timer."
elif [[ -d /etc/cron.d ]]; then
  install -m 0644 "$(dirname "$0")/graceful-rm-cron" /etc/cron.d/graceful-rm
  echo "Installed /usr/local/bin/graceful-rm and /etc/cron.d/graceful-rm."
  echo "Cron will check for trash entries older than three days once per day."
else
  echo "Installed /usr/local/bin/graceful-rm."
  echo "Warning: no systemd or cron was detected; cleanup will run opportunistically on command use."
fi
