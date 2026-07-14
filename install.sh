#!/usr/bin/env bash
set -euo pipefail

HERE="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
hook_agent=""
case " ${*:-} " in
  *" --codex "*) hook_agent=codex ;;
  *" --claude "*) hook_agent=claude ;;
  *" --agy "*|*" --antigravity "*) hook_agent=antigravity ;;
  *" --all "*) hook_agent=all ;;
esac

source_dir=""
if [[ -f "$HERE/scripts/install.sh" ]]; then
  source_dir="$HERE"
else
  repo_url="${GRACEFUL_RM_REPO_URL:-https://github.com/sealifes/graceful-rm}"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  case "$(uname -s)" in
    Linux) platform=linux ;;
    *) echo "install.sh: binary installation currently supports Linux only" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) release_arch=amd64 ;;
    aarch64|arm64) release_arch=arm64 ;;
    *) echo "install.sh: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
  release_base="${GRACEFUL_RM_RELEASE_URL:-${repo_url}/releases/latest/download}"
  download() {
    if command -v curl >/dev/null 2>&1; then
      curl -fsSL "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
      wget -qO "$2" "$1"
    else
      echo "install.sh: curl or wget is required" >&2
      exit 1
    fi
  }
  download "$release_base/graceful-rm-${platform}-${release_arch}" "$tmp_dir/graceful-rm"
  chmod 0755 "$tmp_dir/graceful-rm"
  if [[ -n "$hook_agent" ]]; then
    download "$release_base/graceful-rm-hook-${platform}-${release_arch}" "$tmp_dir/graceful-rm-hook"
    chmod 0755 "$tmp_dir/graceful-rm-hook"
  fi
fi

if [[ -n "$source_dir" ]]; then
  if [[ "${EUID}" -eq 0 ]]; then
    "$source_dir/scripts/install.sh"
  else
    sudo "$source_dir/scripts/install.sh"
  fi
else
  if [[ "${EUID}" -eq 0 ]]; then
    root_cmd=()
  else
    root_cmd=(sudo)
  fi
  if [[ "${EUID}" -eq 0 ]]; then
    install -D -m 0755 "$tmp_dir/graceful-rm" /usr/local/bin/graceful-rm
    if [[ -n "$hook_agent" ]]; then
      install -D -m 0755 "$tmp_dir/graceful-rm-hook" /usr/local/bin/graceful-rm-hook
    fi
  else
    sudo install -D -m 0755 "$tmp_dir/graceful-rm" /usr/local/bin/graceful-rm
    if [[ -n "$hook_agent" ]]; then
      sudo install -D -m 0755 "$tmp_dir/graceful-rm-hook" /usr/local/bin/graceful-rm-hook
    fi
  fi
  echo "Installed /usr/local/bin/graceful-rm from the GitHub release binary."
  if [[ -n "$hook_agent" ]]; then
    echo "Installed /usr/local/bin/graceful-rm-hook for --$hook_agent."
  fi
  if [[ "$(ps -p 1 -o comm= 2>/dev/null || true)" == "systemd" ]] && command -v systemctl >/dev/null 2>&1; then
    printf '%s\n' '[Unit]' 'Description=Delete expired graceful-rm trash entries' '' '[Service]' 'Type=oneshot' 'ExecStart=/usr/local/bin/graceful-rm --cleanup' | "${root_cmd[@]}" install -D -m 0644 /dev/stdin /etc/systemd/system/graceful-rm-cleanup.service
    printf '%s\n' '[Unit]' 'Description=Run graceful-rm cleanup every three days' '' '[Timer]' 'OnBootSec=1h' 'OnUnitActiveSec=3d' 'Persistent=true' 'Unit=graceful-rm-cleanup.service' '' '[Install]' 'WantedBy=timers.target' | "${root_cmd[@]}" install -D -m 0644 /dev/stdin /etc/systemd/system/graceful-rm-cleanup.timer
    "${root_cmd[@]}" systemctl daemon-reload
    "${root_cmd[@]}" systemctl enable --now graceful-rm-cleanup.timer
    echo "Enabled the 3-day systemd cleanup timer."
  elif [[ -d /etc/cron.d ]]; then
    printf '%s\n' '# graceful-rm expiration check' '0 3 * * * root /usr/local/bin/graceful-rm --cleanup >/dev/null 2>&1' | "${root_cmd[@]}" install -m 0644 /dev/stdin /etc/cron.d/graceful-rm
    echo "Installed the /etc/cron.d/graceful-rm cleanup job."
  else
    echo "Warning: no systemd or cron was detected; cleanup will run opportunistically."
  fi
fi

if [[ -n "$hook_agent" ]]; then
  if [[ -n "$source_dir" ]]; then
    "$source_dir/scripts/install-hooks.sh" --agent "$hook_agent"
  else
    /usr/local/bin/graceful-rm "--$hook_agent"
  fi
fi
