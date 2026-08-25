#!/usr/bin/env bash
# One-liner entrypoint for MuxCore guided onboarding.
# Usage:
#   curl -fsSL https://getmuxcore.zem.systems | bash
#   GITHUB_TOKEN=ghp_... curl -fsSL https://getmuxcore.zem.systems | bash
#   bash get-onboard.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || pwd)"
MUXCORE_INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-Muxcore-Media/muxcore-installer}"
MUXCORE_INSTALLER_REF="${MUXCORE_INSTALLER_REF:-main}"
DEFAULT_INSTALL_DIR="${MUXCORE_INSTALL_DIR:-$HOME/muxcore}"

die() { echo "error: $*" >&2; exit 1; }

_BOOTSTRAP_READ_FD=0
bootstrap_open_tty() {
  if [[ -r /dev/tty ]]; then
    exec 3</dev/tty
    _BOOTSTRAP_READ_FD=3
    return 0
  fi
  if [[ ! -t 0 ]]; then
    die "interactive setup requires a terminal. Run: curl -fsSL https://getmuxcore.zem.systems -o get-onboard.sh && bash get-onboard.sh"
  fi
  _BOOTSTRAP_READ_FD=0
  return 0
}

bootstrap_prompt() {
  local var="$1" q="$2" def="${3:-}" ans
  bootstrap_open_tty
  read -r -p "$q [$def]: " ans -u "$_BOOTSTRAP_READ_FD" || true
  printf -v "$var" '%s' "${ans:-$def}"
}

bootstrap_ensure_writable_dir() {
  local d="$1"
  [[ -e "$d" && ! -d "$d" ]] && return 1
  mkdir -p "$d" 2>/dev/null || return 1
  [[ -w "$d" ]]
}

bootstrap_resolve_dir() {
  local raw="$1" resolved
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

bootstrap_github_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then printf '%s' "$GITHUB_TOKEN"; return 0; fi
  if [[ -n "${GH_TOKEN:-}" ]]; then printf '%s' "$GH_TOKEN"; return 0; fi
  local f
  for f in "$HOME/.config/muxcore/github.token" "$HOME/.config/gh/hosts.yml"; do
    [[ -f "$f" ]] || continue
    [[ -r "$f" ]] || { echo "warning: cannot read $f (permission denied)" >&2; continue; }
    if [[ "$f" == *hosts.yml ]]; then
      awk '/oauth_token:/ {print $2; exit}' "$f" 2>/dev/null && return 0
      continue
    fi
    tr -d '[:space:]' <"$f"
    return 0
  done
  command -v gh >/dev/null 2>&1 && gh auth token 2>/dev/null && return 0
  return 1
}

piped_bootstrap() { [[ ! -t 0 ]]; }

detect_os() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux|darwin) printf '%s\n' "$os" ;;
    *) die "unsupported OS: $os (MuxCore onboarding supports Linux and macOS)" ;;
  esac
}

find_local_installer() {
  if piped_bootstrap; then
    return 1
  fi
  local here src
  here="$SCRIPT_DIR"
  if [[ -f "$here/onboard.sh" && -f "$here/lib/common.sh" ]]; then
    printf '%s\n' "$here"
    return 0
  fi
  for src in \
    "$here/muxcore-installer" \
    "$here/../muxcore-installer" \
    "$PWD/muxcore-installer" \
    "$PWD"; do
    if [[ -f "$src/onboard.sh" && -f "$src/lib/common.sh" ]]; then
      cd "$src" && pwd
      return 0
    fi
  done
  return 1
}

fetch_installer() {
  local dest="$1"
  bootstrap_ensure_writable_dir "$dest" || die "cannot use install folder $dest (permission denied)"

  if [[ -e "$dest" && ! -d "$dest/.git" && -n "$(ls -A "$dest" 2>/dev/null || true)" ]]; then
    die "$dest exists and is not empty — choose another folder or remove it first"
  fi

  local token clone_url
  token="$(bootstrap_github_token 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    clone_url="https://x-access-token:${token}@github.com/${MUXCORE_INSTALLER_REPO}.git"
  else
    clone_url="https://github.com/${MUXCORE_INSTALLER_REPO}.git"
  fi

  if command -v git >/dev/null 2>&1; then
    if [[ -d "$dest/.git" ]]; then
      echo "==> updating installer in $dest"
      git -C "$dest" fetch --depth 1 origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 \
        || die "git fetch failed (check GITHUB_TOKEN for private repos)"
      git -C "$dest" checkout "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 || true
      git -C "$dest" pull --ff-only origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 \
        || die "git pull failed in $dest"
      return 0
    fi
    echo "==> cloning ${MUXCORE_INSTALLER_REPO} (${MUXCORE_INSTALLER_REF}) → $dest"
    GIT_TERMINAL_PROMPT=0 git clone --depth 1 --branch "$MUXCORE_INSTALLER_REF" "$clone_url" "$dest" \
      || die "git clone failed (set GITHUB_TOKEN for private repos or check folder permissions)"
    return 0
  fi

  command -v curl >/dev/null 2>&1 || die "need curl or git to fetch the installer"
  command -v tar >/dev/null 2>&1 || die "need tar to extract the installer"
  local tmp archive="muxcore-installer-${MUXCORE_INSTALLER_REF}.tar.gz"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/muxcore-onboard.XXXXXX")"
  echo "==> downloading installer archive from GitHub"
  if [[ -n "$token" ]]; then
    curl -fsSL -H "Authorization: Bearer ${token}" \
      -o "$tmp/$archive" \
      "https://api.github.com/repos/${MUXCORE_INSTALLER_REPO}/tarball/${MUXCORE_INSTALLER_REF}" \
      || die "could not download installer"
  else
    curl -fsSL -o "$tmp/$archive" \
      "https://api.github.com/repos/${MUXCORE_INSTALLER_REPO}/tarball/${MUXCORE_INSTALLER_REF}" \
      || die "could not download installer (set GITHUB_TOKEN for private repos)"
  fi
  tar -xzf "$tmp/$archive" -C "$tmp"
  local extracted
  extracted="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d | head -1)"
  [[ -n "$extracted" && -f "$extracted/onboard.sh" ]] || die "archive did not contain onboard.sh"
  if command -v rsync >/dev/null 2>&1; then
    rsync -a "$extracted/" "$dest/"
  else
    mkdir -p "$dest"
    tar -C "$extracted" -cf - . | tar -C "$dest" -xf -
  fi
  rm -rf "$tmp"
}

main() {
  detect_os >/dev/null
  command -v bash >/dev/null 2>&1 || die "bash is required"
  bootstrap_open_tty

  echo
  echo "MuxCore guided setup"
  echo "────────────────────"
  echo "Interactive walkthrough: install folder, libraries, admin account, and optional extras."
  echo "You will be asked questions at each step."
  echo

  local installer_root target
  if installer_root="$(find_local_installer)"; then
    echo "==> using local installer at $installer_root"
  else
    bootstrap_prompt target "Install folder" "$DEFAULT_INSTALL_DIR"
    target="$(bootstrap_resolve_dir "$target")"
    fetch_installer "$target"
    installer_root="$target"
  fi

  [[ -f "$installer_root/onboard.sh" ]] || die "onboard.sh missing in $installer_root"
  bootstrap_ensure_writable_dir "$installer_root" || die "installer directory not writable: $installer_root"

  for script in onboard.sh install.sh up.sh bootstrap-auth.sh smoke-fixture.sh; do
    [[ -f "$installer_root/$script" ]] && chmod u+x "$installer_root/$script" 2>/dev/null || true
  done

  exec bash "$installer_root/onboard.sh" "$@"
}

main "$@"
