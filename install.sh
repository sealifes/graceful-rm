#!/usr/bin/env bash
set -euo pipefail

HERE="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$HERE/scripts/install.sh" ]]; then
  exec "$HERE/scripts/install.sh" "$@"
fi

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
if [[ -z "$source_dir" || ! -f "$source_dir/scripts/install.sh" ]]; then
  echo "install.sh: downloaded archive has an unexpected layout" >&2
  exit 1
fi
exec "$source_dir/scripts/install.sh" "$@"
