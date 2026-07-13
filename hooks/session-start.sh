#!/usr/bin/env bash
set -euo pipefail

# Session hooks must be non-destructive and must not request root privileges.
if command -v graceful-rm >/dev/null 2>&1; then
  timer_state="not-installed"
  if command -v systemctl >/dev/null 2>&1; then
    timer_state="$(systemctl is-enabled graceful-rm-cleanup.timer 2>/dev/null || true)"
  fi
  printf 'graceful-rm: installed; cleanup timer: %s\n' "${timer_state:-unknown}"
else
  printf 'graceful-rm: not installed; run the plugin install script with root privileges.\n'
fi
