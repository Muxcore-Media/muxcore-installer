#!/usr/bin/env bash
# Fetch GitHub Release binaries for ENABLED_MODULES and write .env / VIEW-ME.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/github.sh"
# shellcheck disable=SC1091
source "$ROOT/lib/modules.sh"
# shellcheck disable=SC1091
source "$ROOT/versions.env"

BIN="$ROOT/bin"
CACHE="$ROOT/cache/releases"
DATA="$ROOT/data"
RUN="$ROOT/run"

require_cmd curl tar uname mkdir chmod

mkdir -p "$BIN" "$CACHE" "$RUN" \
  "$DATA"/{movies,tvshows,scanner,roots,sqlite,secrets,encryption,library/tv,library/music,storage,auth,import,formats,rename,ffprobe,subtitles/files}

if ! ensure_writable_dir "$BIN" 2>/dev/null; then
  echo "error: cannot write to $BIN (permission denied)" >&2
  exit 1
fi

if [[ -f "$ROOT/.env" ]]; then
  # shellcheck disable=SC1091
  source "$ROOT/.env" || true
fi

if [[ -z "${ENABLED_MODULES:-}" ]]; then
  ENABLED_MODULES="$(resolve_enabled_modules "${MUXCORE_LIBRARIES:-Movies,TV}" "${MUXCORE_PLAYBACK:-}" "${MUXCORE_PROFILE:-sqlite}")"
fi
export ENABLED_MODULES

read -r OS ARCH < <(detect_os_arch)
echo "==> platform ${OS}/${ARCH}"
echo "==> modules: $ENABLED_MODULES"

gh_available() { command -v gh >/dev/null 2>&1 && github_token >/dev/null 2>&1; }

extract_binary_from_tarball() {
  local tarball="$1" want_name="$2"
  local tmp
  tmp="$(mktemp -d "$CACHE/extract.XXXXXX")"
  tar -xzf "$tarball" -C "$tmp"
  local found=""
  if [[ -f "$tmp/$want_name" ]]; then
    found="$tmp/$want_name"
  else
    found="$(find "$tmp" -type f -name "$want_name" | head -1 || true)"
  fi
  if [[ -z "$found" ]]; then
    found="$(find "$tmp" -type f ! -name 'LICENSE*' ! -name 'README*' ! -name '*.txt' ! -name '*.md' | head -1 || true)"
  fi
  if [[ -z "$found" || ! -f "$found" ]]; then
    rm -rf "$tmp"
    return 1
  fi
  install -m 0755 "$found" "$BIN/$want_name"
  rm -rf "$tmp"
  echo "    installed bin/$want_name"
  return 0
}

try_release_asset() {
  local repo="$1" tag="$2" asset_prefix="$3" bin_name="$4"
  local ver="${tag#v}"
  local gh_repo="${GITHUB_ORG:-$(github_org)}/${repo}"
  local candidates="
${asset_prefix}_${ver}_${OS}_${ARCH}.tar.gz
${bin_name}_${ver}_${OS}_${ARCH}.tar.gz
${asset_prefix}_${OS}_${ARCH}.tar.gz
${bin_name}_${OS}_${ARCH}.tar.gz
"

  if gh_available; then
    local ghtmp
    ghtmp="$(mktemp -d "$CACHE/gh.XXXXXX")"
    if gh release download "$tag" --repo "$gh_repo" --dir "$ghtmp" -p "*${OS}_${ARCH}*" >/dev/null 2>&1 \
      || gh release download "$tag" --repo "$gh_repo" --dir "$ghtmp" -p "*.tar.gz" >/dev/null 2>&1; then
      local tarball
      tarball="$(find "$ghtmp" -type f \( -name '*.tar.gz' -o -name '*.tgz' \) | head -1 || true)"
      if [[ -n "$tarball" ]] && extract_binary_from_tarball "$tarball" "$bin_name"; then
        rm -rf "$ghtmp"
        return 0
      fi
      if [[ -f "$ghtmp/$bin_name" ]]; then
        install -m 0755 "$ghtmp/$bin_name" "$BIN/$bin_name"
        rm -rf "$ghtmp"
        echo "    installed bin/$bin_name (gh asset)"
        return 0
      fi
    fi
    rm -rf "$ghtmp"
  fi

  local asset dest
  while IFS= read -r asset; do
    [[ -n "$asset" ]] || continue
    dest="$CACHE/${repo}-${tag}-${asset}"
    if [[ -f "$dest" ]]; then
      echo "    cached $(basename "$dest")"
    elif github_download_release_asset "$repo" "$tag" "$asset" "$dest"; then
      echo "    downloaded $(basename "$dest")"
    else
      continue
    fi
    if extract_binary_from_tarball "$dest" "$bin_name"; then
      return 0
    fi
    echo "    WARN: tarball $asset had no usable binary named $bin_name" >&2
  done <<EOF
$candidates
EOF
  return 1
}

copy_from_lab() {
  local name="$1"
  local lab
  if ! lab="$(resolve_lab_bin "$ROOT")"; then
    return 1
  fi
  if [[ -x "$lab/$name" ]]; then
    install -m 0755 "$lab/$name" "$BIN/$name"
    echo "    copied bin/$name from lab $lab"
    return 0
  fi
  return 1
}

repo_for_bin() {
  local name="$1"
  case "$name" in
    muxcored) printf '%s\n' core ;;
    mediauiprox) printf '%s\n' media-ui ;;
    *) printf '%s\n' "$name" ;;
  esac
}

MISSING=()
LAB_COPIED=()

fetch_one() {
  local bin_name="$1"
  local repo tag
  if [[ -x "$BIN/$bin_name" ]]; then
    echo "  - $bin_name already present"
    return 0
  fi
  repo="$(repo_for_bin "$bin_name")"
  tag="$(module_tag "$bin_name")"
  echo "  - ${repo}@${tag} → $bin_name"
  if try_release_asset "$repo" "$tag" "$bin_name" "$bin_name"; then
    return 0
  fi
  if [[ "$bin_name" == muxcored ]]; then
    if try_release_asset "$CORE_REPO" "$CORE_TAG" "$CORE_ASSET_PREFIX" muxcored; then
      return 0
    fi
  fi
  if copy_from_lab "$bin_name"; then
    LAB_COPIED+=("$bin_name")
    return 0
  fi
  MISSING+=("$bin_name@$tag")
  echo "    MISSING: no GitHub Release binary for ${repo}@${tag}" >&2
  return 1
}

echo "==> downloading selected modules"
for m in $ENABLED_MODULES; do
  fetch_one "$m" || true
done

echo "==> helper CLIs (${HELPER_BINS})"
for h in $HELPER_BINS; do
  if [[ -x "$BIN/$h" ]]; then
    echo "  - $h already present"
    continue
  fi
  if [[ "$h" == authctl ]] && try_release_asset "auth-local" "$(module_tag auth-local)" authctl authctl; then
    continue
  fi
  if [[ "$h" == gettoken ]] && try_release_asset "muxcorectl-cli" "v0.1.0" gettoken gettoken; then
    continue
  fi
  if copy_from_lab "$h"; then
    LAB_COPIED+=("$h")
    continue
  fi
  MISSING+=("$h")
  echo "  - MISSING helper $h" >&2
done

echo "==> writing .env"
if [[ ! -f "$ROOT/.env" ]]; then
  cp "$ROOT/.env.example" "$ROOT/.env"
fi
# shellcheck disable=SC1091
source "$ROOT/.env"
write_view_me "$ROOT"

ESSENTIAL="muxcored api-rest auth-local secrets-file admin-ui"
if module_enabled database-postgres; then
  ESSENTIAL="$ESSENTIAL database-postgres"
else
  ESSENTIAL="$ESSENTIAL database-sqlite"
fi
ESSENTIAL_OK=1
for e in $ESSENTIAL; do
  if [[ ! -x "$BIN/$e" ]]; then
    ESSENTIAL_OK=0
    echo "FAIL: missing essential binary bin/$e" >&2
  fi
done

echo
echo "======== install summary ========"
echo "root:     $ROOT"
echo "bin:      $BIN"
echo "platform: ${OS}/${ARCH}"
if [[ ${#LAB_COPIED[@]} -gt 0 ]]; then
  echo "lab copies: ${LAB_COPIED[*]}"
fi
if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "MISSING:  ${MISSING[*]}"
  echo "Place release binaries in bin/ or set MUXCORE_LAB_BIN, then re-run ./install.sh"
fi
echo "VIEW-ME: $RUN/VIEW-ME.txt"
if [[ "$ESSENTIAL_OK" -ne 1 ]]; then
  exit 1
fi
exit 0
