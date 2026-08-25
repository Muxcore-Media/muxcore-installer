#!/usr/bin/env bash
# Bootstrap entry: fetch installer repo if needed, then run onboard.sh.
# Usage:
#   curl -fsSL https://getmuxcore.zem.systems | bash
#   bash get-onboard.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || pwd)"
MUXCORE_INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-Muxcore-Media/muxcore-installer}"
MUXCORE_INSTALLER_REF="${MUXCORE_INSTALLER_REF:-main}"
DEFAULT_INSTALL_DIR="${MUXCORE_INSTALL_DIR:-$HOME/muxcore}"

die() { echo "error: $*" >&2; exit 1; }

piped_bootstrap() { [[ ! -t 0 ]]; }

bootstrap_github_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then printf '%s' "$GITHUB_TOKEN"; return 0; fi
  if [[ -n "${GH_TOKEN:-}" ]]; then printf '%s' "$GH_TOKEN"; return 0; fi
  local f
  for f in "$HOME/.config/muxcore/github.token" "$HOME/.config/gh/hosts.yml"; do
    [[ -f "$f" && -r "$f" ]] || continue
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

find_local_installer() {
  piped_bootstrap && return 1
  local here="$SCRIPT_DIR" src
  if [[ -f "$here/onboard.sh" && -f "$here/lib/ui.sh" ]]; then
    printf '%s\n' "$here"
    return 0
  fi
  for src in "$here/muxcore-installer" "$here/../muxcore-installer" "$PWD/muxcore-installer" "$PWD"; do
    if [[ -f "$src/onboard.sh" && -f "$src/lib/ui.sh" ]]; then
      cd "$src" && pwd
      return 0
    fi
  done
  return 1
}

fetch_installer() {
  local dest="$1"
  mkdir -p "$dest" 2>/dev/null || die "cannot create $dest (permission denied)"
  [[ -w "$dest" ]] || die "directory not writable: $dest"

  if [[ -e "$dest" && ! -d "$dest/.git" && -n "$(ls -A "$dest" 2>/dev/null || true)" ]]; then
    die "$dest exists and is not empty — remove it or set MUXCORE_INSTALL_DIR elsewhere"
  fi

  local token clone_url
  token="$(bootstrap_github_token 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    clone_url="https://x-access-token:${token}@github.com/${MUXCORE_INSTALLER_REPO}.git"
  else
    clone_url="https://github.com/${MUXCORE_INSTALLER_REPO}.git"
  fi

  if ! command -v git >/dev/null 2>&1; then
    die "git is required — run onboard.sh from a full checkout or install git first"
  fi

  if [[ -d "$dest/.git" ]]; then
    echo "==> updating installer in $dest"
    git -C "$dest" fetch --depth 1 origin "$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 \
      || die "git fetch failed (set GITHUB_TOKEN for private repos)"
    git -C "$dest" checkout -q "$MUXCORE_INSTALLER_REF" 2>/dev/null \
      || git -C "$dest" checkout -q -B "$MUXCORE_INSTALLER_REF" "origin/$MUXCORE_INSTALLER_REF"
    git -C "$dest" reset --hard "origin/$MUXCORE_INSTALLER_REF" >/dev/null 2>&1 \
      || die "git reset failed in $dest"
    return 0
  fi

  echo "==> cloning ${MUXCORE_INSTALLER_REPO} (${MUXCORE_INSTALLER_REF}) → $dest"
  GIT_TERMINAL_PROMPT=0 git clone --depth 1 --branch "$MUXCORE_INSTALLER_REF" "$clone_url" "$dest" \
    || die "git clone failed (set GITHUB_TOKEN for private repos)"
}

main() {
  command -v bash >/dev/null 2>&1 || die "bash is required"

  local installer_root target
  if installer_root="$(find_local_installer)"; then
    :
  else
    target="$(bootstrap_resolve_dir "${MUXCORE_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}")"
    fetch_installer "$target"
    installer_root="$target"
  fi

  [[ -f "$installer_root/onboard.sh" ]] || die "onboard.sh missing in $installer_root"
  for script in onboard.sh install.sh up.sh bootstrap-auth.sh smoke-fixture.sh; do
    [[ -f "$installer_root/$script" ]] && chmod u+x "$installer_root/$script" 2>/dev/null || true
  done

  exec bash "$installer_root/onboard.sh" "$@"
}

main "$@"
