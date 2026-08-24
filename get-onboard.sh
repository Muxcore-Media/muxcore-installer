#!/usr/bin/env bash
# One-liner entrypoint for MuxCore guided onboarding.
# Usage:
#   curl -fsSL https://getmuxcore.zem.systems | bash
#   GITHUB_TOKEN=ghp_... curl -fsSL https://getmuxcore.zem.systems | bash
#   bash get-onboard.sh
set -euo pipefail

MUXCORE_INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-Muxcore-Media/muxcore-installer}"
MUXCORE_INSTALLER_REF="${MUXCORE_INSTALLER_REF:-main}"
DEFAULT_INSTALL_DIR="${MUXCORE_INSTALL_DIR:-$HOME/muxcore}"
GITHUB_ORG="${MUXCORE_GITHUB_ORG:-Muxcore-Media}"

die() { echo "error: $*" >&2; exit 1; }

github_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then printf '%s' "$GITHUB_TOKEN"; return 0; fi
  if [[ -n "${GH_TOKEN:-}" ]]; then printf '%s' "$GH_TOKEN"; return 0; fi
  if [[ -f "$HOME/.config/muxcore/github.token" ]]; then tr -d '[:space:]' <"$HOME/.config/muxcore/github.token"; return 0; fi
  if command -v gh >/dev/null 2>&1; then gh auth token 2>/dev/null && return 0; fi
  return 1
}

github_authed_curl() {
  local url="$1" dest="$2" token
  if token="$(github_token 2>/dev/null || true)" && [[ -n "$token" ]]; then
    curl -fsSL -H "Authorization: Bearer ${token}" -o "$dest" "$url"
  else
    curl -fsSL -o "$dest" "$url"
  fi
}

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
  local token clone_url
  token="$(github_token 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    clone_url="https://x-access-token:${token}@github.com/${MUXCORE_INSTALLER_REPO}.git"
  else
    clone_url="https://github.com/${MUXCORE_INSTALLER_REPO}.git"
  fi

  if command -v git >/dev/null 2>&1; then
    if [[ -d "$dest/.git" ]]; then
      echo "==> updating installer in $dest"
      git -C "$dest" fetch --depth 1 origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      git -C "$dest" checkout "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      git -C "$dest" pull --ff-only origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      return 0
    fi
    echo "==> cloning ${MUXCORE_INSTALLER_REPO} (${MUXCORE_INSTALLER_REF}) → $dest"
    git clone --depth 1 --branch "$MUXCORE_INSTALLER_REF" "$clone_url" "$dest"
    return 0
  fi

  command -v curl >/dev/null 2>&1 || die "need curl or git to fetch the installer"
  command -v tar >/dev/null 2>&1 || die "need tar to extract the installer"
  local tmp archive="muxcore-installer-${MUXCORE_INSTALLER_REF}.tar.gz"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/muxcore-onboard.XXXXXX")"
  echo "==> downloading installer archive from GitHub"
  github_authed_curl \
    "https://api.github.com/repos/${MUXCORE_INSTALLER_REPO}/tarball/${MUXCORE_INSTALLER_REF}" \
    "$tmp/$archive" || die "could not download installer (set GITHUB_TOKEN for private repos)"
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
