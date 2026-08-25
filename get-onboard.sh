#!/usr/bin/env bash
# Landing script for: curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
# Downloads a pinned installer tarball (checksummed when published), then execs onboard.sh.
# No sudo. No package installs. No module downloads.
set -euo pipefail

# Baked pin — override with MUXCORE_INSTALLER_TAG. Not "latest".
INSTALLER_TAG="${MUXCORE_INSTALLER_TAG:-v0.2.0}"
INSTALLER_REPO="${MUXCORE_INSTALLER_REPO:-Muxcore-Media/muxcore-installer}"
GITHUB_ORG="${MUXCORE_GITHUB_ORG:-Muxcore-Media}"

die() { echo "error: $*" >&2; exit 1; }

refuse_windows() {
  case "$(uname -s 2>/dev/null || echo unknown)" in
    MINGW*|MSYS*|CYGWIN*|Windows_NT|windows)
      cat >&2 <<'EOF'
error: native Windows is not supported.
Install MuxCore inside WSL2 (Ubuntu), then re-run:

  curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
EOF
      exit 1
      ;;
  esac
}

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "unsupported architecture: $arch (need amd64 or arm64)" ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *) die "unsupported OS: $os (need Linux or macOS)" ;;
  esac
  printf '%s %s\n' "$os" "$arch"
}

rebind_tty() {
  if [[ -t 0 ]]; then
    return 0
  fi
  if [[ -r /dev/tty ]] && exec </dev/tty 2>/dev/null; then
    return 0
  fi
  if [[ "${MUXCORE_NONINTERACTIVE:-}" == "1" ]]; then
    return 0
  fi
  cat >&2 <<'EOF'
error: this installer needs a terminal (stdin is not a TTY).

Safer two-step:

  curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
  bash get-muxcore.sh
EOF
  exit 1
}

bootstrap_token() {
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then printf '%s' "$GITHUB_TOKEN"; return 0; fi
  if [[ -n "${GH_TOKEN:-}" ]]; then printf '%s' "$GH_TOKEN"; return 0; fi
  if [[ -n "${MUXCORE_GITHUB_TOKEN:-}" ]]; then printf '%s' "$MUXCORE_GITHUB_TOKEN"; return 0; fi
  local f
  for f in \
    "${MUXCORE_GITHUB_TOKEN_FILE:-}" \
    "$HOME/.config/muxcore/github.token"; do
    [[ -n "$f" && -f "$f" && -r "$f" ]] || continue
    tr -d '[:space:]' <"$f"
    return 0
  done
  if command -v gh >/dev/null 2>&1; then
    gh auth token 2>/dev/null && return 0
  fi
  return 1
}

curl_auth() {
  local token
  token="$(bootstrap_token 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    curl --proto '=https' --tlsv1.2 -fsSL \
      -H "Authorization: Bearer ${token}" \
      -H "Accept: application/octet-stream" \
      "$@"
  else
    curl --proto '=https' --tlsv1.2 -fsSL "$@"
  fi
}

script_dir() {
  local here
  here="$(dirname "${BASH_SOURCE[0]:-$0}")"
  if here="$(cd "$here" 2>/dev/null && pwd)"; then
    printf '%s\n' "$here"
  else
    pwd
  fi
}

find_local_installer() {
  # Piped curl|bash has no real script dir — never treat CWD as the installer.
  if [[ ! -t 0 && -z "${BASH_SOURCE[0]:-}" ]]; then
    return 1
  fi
  local here src
  here="$(script_dir)"
  if [[ -f "$here/onboard.sh" && -f "$here/lib/ui.sh" ]]; then
    printf '%s\n' "$here"
    return 0
  fi
  for src in \
    "$here/muxcore-installer" \
    "$here/../muxcore-installer" \
    "${PWD:-}/muxcore-installer"; do
    if [[ -f "$src/onboard.sh" && -f "$src/lib/ui.sh" ]]; then
      (cd "$src" && pwd)
      return 0
    fi
  done
  return 1
}

sha256_file() {
  local f="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$f" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$f" | awk '{print $1}'
    return 0
  fi
  return 1
}

verify_sha256() {
  local tarball="$1" sums="$2" want name got
  name="$(basename "$tarball")"
  want="$(awk -v n="$name" '$2==n || $2=="*"n {print $1; exit}' "$sums" 2>/dev/null || true)"
  [[ -n "$want" ]] || { echo "note: no SHA-256 entry for $name — skipping verify" >&2; return 0; }
  got="$(sha256_file "$tarball")" || die "no sha256sum/shasum on PATH to verify download"
  if [[ "$got" != "$want" ]]; then
    die "checksum mismatch for $name (got $got want $want)"
  fi
  echo "==> checksum ok ($name)"
}

extract_and_find() {
  local archive="$1" dest="$2"
  mkdir -p "$dest"
  tar -xzf "$archive" -C "$dest"
  if [[ -f "$dest/onboard.sh" ]]; then
    printf '%s\n' "$dest"
    return 0
  fi
  local found
  found="$(find "$dest" -maxdepth 3 -type f -name onboard.sh 2>/dev/null | head -1 || true)"
  [[ -n "$found" ]] || return 1
  cd "$(dirname "$found")" && pwd
}

fetch_installer() {
  command -v curl >/dev/null 2>&1 || die "curl is required to download the installer"
  command -v tar >/dev/null 2>&1 || die "tar is required to unpack the installer"

  local os arch ver cache dest tarball sums url
  read -r os arch < <(detect_os_arch)
  ver="${INSTALLER_TAG#v}"
  cache="${XDG_CACHE_HOME:-$HOME/.cache}/muxcore-installer/${INSTALLER_TAG}"
  dest="$cache/src"
  mkdir -p "$cache" "$dest"
  tarball="$cache/muxcore-installer_${ver}_${os}_${arch}.tar.gz"
  sums="$cache/SHA256SUMS"

  url="https://github.com/${INSTALLER_REPO}/releases/download/${INSTALLER_TAG}/muxcore-installer_${ver}_${os}_${arch}.tar.gz"
  echo "==> fetching MuxCore installer ${INSTALLER_TAG}"
  if curl_auth -o "$tarball" "$url" 2>/dev/null; then
    curl_auth -o "$sums" \
      "https://github.com/${INSTALLER_REPO}/releases/download/${INSTALLER_TAG}/SHA256SUMS" \
      2>/dev/null || true
    if [[ -s "$sums" ]]; then
      verify_sha256 "$tarball" "$sums"
    fi
    extract_and_find "$tarball" "$dest" && return 0
  fi

  echo "==> release tarball unavailable; trying GitHub archive ${INSTALLER_TAG}"
  url="https://github.com/${INSTALLER_REPO}/archive/refs/tags/${INSTALLER_TAG}.tar.gz"
  if curl_auth -o "$tarball" "$url" 2>/dev/null; then
    extract_and_find "$tarball" "$dest" && return 0
  fi

  echo "==> tag archive unavailable; trying main"
  url="https://codeload.github.com/${GITHUB_ORG}/muxcore-installer/tar.gz/refs/heads/main"
  if curl_auth -o "$tarball" "$url" 2>/dev/null; then
    extract_and_find "$tarball" "$dest" && return 0
  fi

  die "could not download the installer from GitHub (set GITHUB_TOKEN if the repo is still private)"
}

main() {
  command -v bash >/dev/null 2>&1 || die "bash is required"
  refuse_windows
  detect_os_arch >/dev/null
  rebind_tty

  local installer_root
  if installer_root="$(find_local_installer)"; then
    echo "==> using local installer at $installer_root"
  else
    installer_root="$(fetch_installer)"
  fi

  [[ -f "$installer_root/onboard.sh" ]] || die "onboard.sh missing in $installer_root"
  chmod u+x "$installer_root/onboard.sh" \
    "$installer_root/install.sh" \
    "$installer_root/up.sh" \
    "$installer_root/bootstrap-auth.sh" \
    "$installer_root/smoke-fixture.sh" 2>/dev/null || true

  exec bash "$installer_root/onboard.sh" "$@"
}

main "$@"
