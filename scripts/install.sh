#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "install.sh must run as root" >&2
  exit 1
fi

install -D -m 0755 "$(dirname "$0")/graceful-rm" /usr/local/bin/graceful-rm
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
