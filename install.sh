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

if [[ -f "$HERE/scripts/install.sh" ]]; then
  source_dir="$HERE"
else
  repo_url="${GRACEFUL_RM_REPO_URL:-https://github.com/sealifes/graceful-rm}"
  ref="${GRACEFUL_RM_REF:-main}"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT
  archive="$tmp_dir/graceful-rm.tar.gz"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${repo_url}/archive/refs/heads/${ref}.tar.gz" -o "$archive"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$archive" "${repo_url}/archive/refs/heads/${ref}.tar.gz"
  else
    echo "install.sh: curl or wget is required for remote installation" >&2
    exit 1
  fi
  tar -xzf "$archive" -C "$tmp_dir"
  source_dir="$(find "$tmp_dir" -mindepth 1 -maxdepth 1 -type d -name 'graceful-rm-*' -print -quit)"
fi

if [[ -z "$source_dir" || ! -f "$source_dir/scripts/install.sh" ]]; then
  echo "install.sh: downloaded archive has an unexpected layout" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1 && [[ ! -x "$source_dir/bin/graceful-rm" ]]; then
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) release_arch=amd64 ;;
    aarch64|arm64) release_arch=arm64 ;;
    *) echo "install.sh: unsupported architecture without Go: $arch" >&2; exit 1 ;;
  esac
  release_base="${GRACEFUL_RM_RELEASE_URL:-https://github.com/sealifes/graceful-rm/releases/latest/download}"
  mkdir -p "$source_dir/bin"
  download() {
    if command -v curl >/dev/null 2>&1; then curl -fsSL "$1" -o "$2"; else wget -qO "$2" "$1"; fi
  }
  download "$release_base/graceful-rm-linux-$release_arch" "$source_dir/bin/graceful-rm"
  download "$release_base/graceful-rm-hook-linux-$release_arch" "$source_dir/bin/graceful-rm-hook"
  chmod 0755 "$source_dir/bin/graceful-rm" "$source_dir/bin/graceful-rm-hook"
fi

if [[ "${EUID}" -eq 0 ]]; then
  "$source_dir/scripts/install.sh"
else
  sudo "$source_dir/scripts/install.sh"
fi

if [[ -n "$hook_agent" ]]; then
  "$source_dir/scripts/install-hooks.sh" --agent "$hook_agent"
fi
