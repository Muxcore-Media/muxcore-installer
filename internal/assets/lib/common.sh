# Shared helpers for muxcore-installer scripts.
# shellcheck shell=bash

ensure_writable_dir() {
  local d="$1"
  if [[ -e "$d" && ! -d "$d" ]]; then
    echo "error: $d exists and is not a directory" >&2
    return 1
  fi
  if ! mkdir -p "$d" 2>/dev/null; then
    echo "error: cannot create $d (permission denied)" >&2
    return 1
  fi
  if [[ ! -w "$d" ]]; then
    echo "error: directory not writable: $d" >&2
    return 1
  fi
  return 0
}

installer_root() {
  cd "$(dirname "${BASH_SOURCE[1]}")/.." && pwd
}

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *)
      echo "unsupported arch: $arch" >&2
      return 1
      ;;
  esac
  case "$os" in
    linux|darwin) ;;
    *)
      echo "unsupported os: $os" >&2
      return 1
      ;;
  esac
  printf '%s %s\n' "$os" "$arch"
}

resolve_lab_bin() {
  # Prefer explicit override, then sibling MuxCore/_mvp/bin (laptop lab).
  if [[ -n "${MUXCORE_LAB_BIN:-}" ]]; then
    printf '%s\n' "$MUXCORE_LAB_BIN"
    return 0
  fi
  local root="$1"
  local candidate
  for candidate in \
    "$root/../_mvp/bin" \
    "$root/../../_mvp/bin" \
    "$HOME/Projects/MuxCore/_mvp/bin"; do
    if [[ -d "$candidate" ]]; then
      cd "$candidate" && pwd
      return 0
    fi
  done
  return 1
}

require_cmd() {
  local c
  for c in "$@"; do
    command -v "$c" >/dev/null 2>&1 || {
      echo "missing required command: $c" >&2
      return 1
    }
  done
}

write_view_me() {
  local root="$1"
  local out="${2:-$root/run/VIEW-ME.txt}"
  mkdir -p "$(dirname "$out")"
  local user="${MVP_ADMIN_USER:-admin}"
  local pass="${MVP_ADMIN_PASSWORD:-}"
  local player=""
  if [[ "${MVP_ENABLE_MEDIA_UI:-0}" != "0" ]]; then
    player="  Player:       http://127.0.0.1:5173"
  fi
  local health security
  if [[ "${SEC_PROFILE:-household}" == household ]]; then
    health="https://127.0.0.1:8080/health  (curl --cacert mesh/public/ca.crt ...)"
    security="Security:  household profile — TLS on every mesh hop, one enrolled identity per module.
           mesh/ holds the CA key and module keys: never back it up (ADR-0023).
           Lost a module identity? ./bin/muxcored enroll reset <id> --ca-dir mesh/ca"
  else
    health="http://127.0.0.1:8080/health"
    security="Security:  DEV profile — plaintext mesh, development only.
           Switch: re-run the installer with --household (keeps data and logins)."
  fi
  cat >"$out" <<EOF
MuxCore — you're ready

  Admin UI:     http://localhost:8082
                login: ${user} / ${pass}
${player}

  Core health:  ${health}
  REST API:     http://127.0.0.1:18080/api/v1/health
  Monitor:      http://127.0.0.1:9203/status

${security}

  Movie library: ${MVP_LIBRARY_ROOT:-$root/data/library}
  TV library:    ${MVP_TV_LIBRARY_ROOT:-$root/data/library/tv}
  Import folder: ${MVP_IMPORT_DIR:-${MVP_DOWNLOADS_DIR:-$root/data/import}}

Useful admin pages
  /dashboard/monitor
  /modules
  /events?filter=health

Metadata
  TMDB_FIXTURE=${TMDB_FIXTURE:-1} (offline sample titles when no API key)

Start:  ./up.sh
Stop:   ./up.sh stop
Auth:   ./bootstrap-auth.sh
Smoke:  ./smoke-fixture.sh

Admin API token: run/admin.token (muxcorectl / REST when installed)
Re-read:         cat run/VIEW-ME.txt
EOF
  echo "wrote $out"
}

env_set() {
  local envf="$1" key="$2" val="$3"
  local tmp
  tmp="$(mktemp)"
  touch "$envf"
  grep -v "^${key}=" "$envf" >"$tmp" || true
  printf '%s=%s\n' "$key" "$val" >>"$tmp"
  mv "$tmp" "$envf"
}

resolve_install_dir() {
  local raw="$1"
  local resolved
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

# ---- security profile + mesh enrollment (ADR-0016, ADR-0017) ----

# resolve_security_profile FORCE_DEV: sets SEC_PROFILE (dev|household) and
# SEC_SOURCE from FORCE_DEV (1 = --dev), MUXCORE_PROFILE, and the legacy
# insecure flag, with core's rule: an explicit profile wins; unset with
# MUXCORE_INSECURE_DISABLE_TLS=true|1 is dev; unset otherwise is household.
# shellcheck disable=SC2034 # SEC_PROFILE and SEC_SOURCE are read by up.sh
resolve_security_profile() {
  local force_dev="${1:-0}" p insecure
  p="$(printf '%s' "${MUXCORE_PROFILE:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  insecure="$(printf '%s' "${MUXCORE_INSECURE_DISABLE_TLS:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  if [[ "$force_dev" == 1 ]]; then
    SEC_PROFILE=dev
    SEC_SOURCE="--dev"
    return 0
  fi
  case "$p" in
    dev)
      SEC_PROFILE=dev
      SEC_SOURCE="MUXCORE_PROFILE=dev"
      ;;
    household | staging)
      SEC_PROFILE=household
      SEC_SOURCE="MUXCORE_PROFILE=$p"
      ;;
    "" | sqlite | postgres)
      if [[ "$insecure" == true || "$insecure" == 1 ]]; then
        SEC_PROFILE=dev
        SEC_SOURCE="legacy MUXCORE_INSECURE_DISABLE_TLS=true without MUXCORE_PROFILE"
      else
        SEC_PROFILE=household
        SEC_SOURCE="default"
      fi
      ;;
    *)
      echo "FAIL: unknown MUXCORE_PROFILE=${MUXCORE_PROFILE:-} (want household or dev)" >&2
      return 1
      ;;
  esac
  return 0
}

print_dev_banner() {
  cat >&2 <<EOF
================================================================
  DEV SECURITY PROFILE ($1)
  The MuxCore mesh runs WITHOUT TLS and trusts every module by
  the ID it claims. Anyone who can reach the mesh ports can
  impersonate a module. Development only — never for real data.
  Switch: re-run the installer with --household, or set
  MUXCORE_PROFILE=household in .env after reinstalling the
  pinned binaries (README: "Security profile").
================================================================
EOF
}

# valid_enroll_secret SECRET: core's minimum length, and characters that are
# safe unquoted in .env and compose interpolation.
valid_enroll_secret() {
  [[ ${#1} -ge 16 && "$1" =~ ^[A-Za-z0-9._~+/=-]+$ ]]
}

# ensure_enroll_secret ENV_FILE CURRENT: prints the enrollment secret. When
# CURRENT is empty a random one is generated and stored in ENV_FILE (0600)
# once; later runs reuse it.
ensure_enroll_secret() {
  local envf="$1" secret="${2:-}"
  if [[ -z "$secret" ]]; then
    secret="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    env_set "$envf" MUXCORE_ENROLL_SECRET "$secret"
    chmod 600 "$envf"
    echo "==> generated MUXCORE_ENROLL_SECRET (kept in $(basename "$envf"), mode 0600)" >&2
  fi
  if ! valid_enroll_secret "$secret"; then
    echo "FAIL: MUXCORE_ENROLL_SECRET in $envf is invalid (>= 16 characters of [A-Za-z0-9._~+/=-]); remove the line to generate a new one" >&2
    return 1
  fi
  printf '%s\n' "$secret"
}

# mesh_enroll_token SECRET ID: prints ID's single-use enrollment token,
# exactly as core computes it:
#   mct_2_<id>_ + hex(HMAC-SHA256(key = SECRET, msg = ID))
# Uses `muxcored enroll token` when MESH_MUXCORED points at it (it also warns on
# stderr when ID already enrolled in the ledger under MESH_CA_DIR), else
# python3, else openssl. MESH_HMAC=muxcored|python3|openssl forces one (tests).
mesh_enroll_token() {
  local secret="$1" id="$2" impl="${MESH_HMAC:-}" tok="" mac=""
  [[ "$id" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo "invalid module ID: $id" >&2; return 1; }
  if [[ -z "$impl" ]]; then
    if [[ -n "${MESH_MUXCORED:-}" && -x "${MESH_MUXCORED:-}" ]]; then
      impl=muxcored
    elif command -v python3 >/dev/null 2>&1; then
      impl=python3
    else
      impl=openssl
    fi
  fi
  case "$impl" in
    muxcored)
      tok="$(MUXCORE_ENROLL_SECRET="$secret" "$MESH_MUXCORED" enroll token "$id" --ca-dir "${MESH_CA_DIR:-$PWD/mesh/ca}")" || tok=""
      if [[ -z "$tok" && -z "${MESH_HMAC:-}" ]]; then
        # An older muxcored without `enroll`: compute it here.
        MESH_HMAC=python3 mesh_enroll_token "$secret" "$id" 2>/dev/null || MESH_HMAC=openssl mesh_enroll_token "$secret" "$id"
        return
      fi
      ;;
    python3)
      command -v python3 >/dev/null 2>&1 || { echo "python3 not found" >&2; return 1; }
      # The secret travels in the environment, not on a command line.
      mac="$(MESH_HMAC_KEY="$secret" python3 -c 'import hashlib, hmac, os, sys
print(hmac.new(os.environ["MESH_HMAC_KEY"].strip().encode(), sys.argv[1].encode(), hashlib.sha256).hexdigest())' "$id")" || mac=""
      ;;
    openssl)
      command -v openssl >/dev/null 2>&1 || { echo "none of muxcored, python3, openssl available for HMAC-SHA256" >&2; return 1; }
      mac="$(printf '%s' "$id" | openssl dgst -sha256 -hmac "$secret" -r | cut -d' ' -f1)" || mac=""
      ;;
    *)
      echo "MESH_HMAC must be muxcored, python3 or openssl" >&2
      return 1
      ;;
  esac
  [[ -z "$tok" && -n "$mac" ]] && tok="mct_2_${id}_${mac}"
  if [[ ! "$tok" =~ ^mct_2_.+_[0-9a-f]{64}$ ]]; then
    echo "could not compute the enrollment token for $id" >&2
    return 1
  fi
  printf '%s\n' "$tok"
}
