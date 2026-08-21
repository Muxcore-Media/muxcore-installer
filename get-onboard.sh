#!/usr/bin/env bash
# One-liner entrypoint for MuxCore guided onboarding.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Muxcore-Media/muxcore-installer/main/get-onboard.sh | bash
#   bash get-onboard.sh
set -euo pipefail

MUXCORE_INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-https://github.com/Muxcore-Media/muxcore-installer}"
MUXCORE_INSTALLER_REF="${MUXCORE_INSTALLER_REF:-main}"
DEFAULT_INSTALL_DIR="${MUXCORE_INSTALL_DIR:-$HOME/muxcore}"

die() { echo "error: $*" >&2; exit 1; }

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux|darwin) printf '%s\n' "$os" ;;
    *) die "unsupported OS: $os (MuxCore onboarding supports Linux and macOS)" ;;
  esac
}

resolve_dir() {
  local raw="$1"
  if resolved="$(cd / && realpath -m "$raw" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
  elif resolved="$(python3 -c 'import os,sys; print(os.path.abspath(os.path.expanduser(sys.argv[1])))' "$raw" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
  else
    case "$raw" in
      ~/*) printf '%s\n' "${HOME}/${raw#~/}" ;;
      ~) printf '%s\n' "$HOME" ;;
      *) printf '%s\n' "$raw" ;;
    esac
  fi
}

find_local_installer() {
  local here src
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  if [[ -x "$here/onboard.sh" ]]; then
    printf '%s\n' "$here"
    return 0
  fi
  for src in \
    "$here/muxcore-installer" \
    "$here/../muxcore-installer" \
    "$PWD/muxcore-installer" \
    "$PWD"; do
    if [[ -x "$src/onboard.sh" ]]; then
      cd "$src" && pwd
      return 0
    fi
  done
  return 1
}

fetch_installer() {
  local dest="$1"
  mkdir -p "$dest"
  if command -v git >/dev/null 2>&1; then
    if [[ -d "$dest/.git" ]]; then
      echo "==> updating installer in $dest"
      git -C "$dest" fetch --depth 1 origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      git -C "$dest" checkout "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      git -C "$dest" pull --ff-only origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      return 0
    fi
    echo "==> cloning $MUXCORE_INSTALLER_REPO ($MUXCORE_INSTALLER_REF) → $dest"
    git clone --depth 1 --branch "$MUXCORE_INSTALLER_REF" "$MUXCORE_INSTALLER_REPO" "$dest"
    return 0
  fi

  command -v curl >/dev/null 2>&1 || die "need curl or git to fetch the installer"
  command -v tar >/dev/null 2>&1 || die "need tar to extract the installer"
  local tmp archive="muxcore-installer-${MUXCORE_INSTALLER_REF}.tar.gz"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/muxcore-onboard.XXXXXX")"
  echo "==> downloading installer archive"
  curl -fsSL \
    "${MUXCORE_INSTALLER_REPO}/archive/refs/heads/${MUXCORE_INSTALLER_REF}.tar.gz" \
    -o "$tmp/$archive"
  tar -xzf "$tmp/$archive" -C "$tmp"
  local extracted
  extracted="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d | head -1)"
  [[ -n "$extracted" && -f "$extracted/onboard.sh" ]] || die "archive did not contain onboard.sh"
  rsync -a "$extracted/" "$dest/" 2>/dev/null || {
    mkdir -p "$dest"
    tar -C "$extracted" -cf - . | tar -C "$dest" -xf -
  }
  rm -rf "$tmp"
}

main() {
  detect_os >/dev/null
  command -v bash >/dev/null 2>&1 || die "bash is required"

  echo
  echo "MuxCore guided setup"
  echo "────────────────────"
  echo "One-line install for Linux and macOS."
  echo

  local installer_root target ans
  if installer_root="$(find_local_installer)"; then
    echo "==> using local installer at $installer_root"
  else
    read -r -p "Install folder [$DEFAULT_INSTALL_DIR]: " ans || true
    target="$(resolve_dir "${ans:-$DEFAULT_INSTALL_DIR}")"
    fetch_installer "$target"
    installer_root="$target"
  fi

  chmod +x "$installer_root/onboard.sh" "$installer_root/install.sh" \
    "$installer_root/up.sh" "$installer_root/bootstrap-auth.sh" \
    "$installer_root/smoke-fixture.sh" 2>/dev/null || true

  exec bash "$installer_root/onboard.sh" "$@"
}

main "$@"
