#!/usr/bin/env bash
# MuxCore single-machine installer: fetch release assets, lay out dirs, write .env + VIEW-ME.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
# shellcheck disable=SC1091
source "$ROOT/lib/common.sh"
# shellcheck disable=SC1091
source "$ROOT/versions.env"

BIN="$ROOT/bin"
CACHE="$ROOT/cache/releases"
DATA="$ROOT/data"
RUN="$ROOT/run"

require_cmd curl tar uname mkdir chmod

mkdir -p "$BIN" "$CACHE" "$RUN" \
  "$DATA"/{movies,tvshows,automation,scanner,roots,sqlite,secrets,encryption,library/tv,storage,auth,jellyfin,downloads,request}

read -r OS ARCH < <(detect_os_arch)
echo "==> platform ${OS}/${ARCH}"

gh_available() { command -v gh >/dev/null 2>&1; }

download_url() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "    cached $(basename "$dest")"
    return 0
  fi
  # Quiet on 404 (private repos / missing assets); caller tries gh + lab fallback.
  if curl -fsSL -o "$dest.partial" "$url" 2>/dev/null; then
    mv "$dest.partial" "$dest"
    echo "    downloaded $(basename "$dest")"
    return 0
  fi
  rm -f "$dest.partial"
  return 1
}

# Extract first executable-looking binary from a tarball into $BIN/$name
extract_binary_from_tarball() {
  local tarball="$1" want_name="$2"
  local tmp
  tmp="$(mktemp -d "$CACHE/extract.XXXXXX")"
  tar -xzf "$tarball" -C "$tmp"
  local found=""
  # Prefer exact name match anywhere in the archive.
  if [[ -f "$tmp/$want_name" ]]; then
    found="$tmp/$want_name"
  else
    found="$(find "$tmp" -type f -name "$want_name" | head -1 || true)"
  fi
  if [[ -z "$found" ]]; then
    # Single-file archive fallback: take the only non-LICENSE/README file.
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
  local candidates=(
    "${asset_prefix}_${ver}_${OS}_${ARCH}.tar.gz"
    "${bin_name}_${ver}_${OS}_${ARCH}.tar.gz"
    "${asset_prefix}_${OS}_${ARCH}.tar.gz"
    "${bin_name}_${OS}_${ARCH}.tar.gz"
  )

  # Prefer gh for private Muxcore-Media releases (curl gets 404 without token).
  if gh_available; then
    local ghtmp
    ghtmp="$(mktemp -d "$CACHE/gh.XXXXXX")"
    if gh release download "$tag" --repo "$repo" --dir "$ghtmp" -p "*${OS}_${ARCH}*" >/dev/null 2>&1 \
      || gh release download "$tag" --repo "$repo" --dir "$ghtmp" -p "*.tar.gz" >/dev/null 2>&1; then
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

  local asset url dest
  for asset in "${candidates[@]}"; do
    dest="$CACHE/${repo##*/}-${tag}-${asset}"
    url="https://github.com/${repo}/releases/download/${tag}/${asset}"
    if download_url "$url" "$dest"; then
      if extract_binary_from_tarball "$dest" "$bin_name"; then
        return 0
      fi
      echo "    WARN: tarball $asset had no usable binary named $bin_name" >&2
    fi
  done
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

MISSING=()
LAB_COPIED=()

echo "==> core ${CORE_REPO}@${CORE_TAG}"
if ! try_release_asset "$CORE_REPO" "$CORE_TAG" "$CORE_ASSET_PREFIX" muxcored; then
  if copy_from_lab muxcored; then
    LAB_COPIED+=(muxcored)
  else
    MISSING+=(muxcored)
  fi
fi

echo "==> modules (release assets, then laptop lab fallback)"
while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  repo_name="${line%%=*}"
  tag="${line#*=}"
  bin_name="$repo_name"
  echo "  - ${repo_name}@${tag}"
  if [[ -x "$BIN/$bin_name" ]]; then
    echo "    already present"
    continue
  fi
  if try_release_asset "Muxcore-Media/${repo_name}" "$tag" "$repo_name" "$bin_name"; then
    continue
  fi
  if copy_from_lab "$bin_name"; then
    LAB_COPIED+=("$bin_name")
    continue
  fi
  MISSING+=("$bin_name@$tag")
  echo "    MISSING: no GitHub Release binary for ${repo_name}@${tag} and no lab bin" >&2
done <<<"$MODULES"

AUTH_LOCAL_TAG=""
while IFS= read -r line; do
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  if [[ "${line%%=*}" == "auth-local" ]]; then
    AUTH_LOCAL_TAG="${line#*=}"
    break
  fi
done <<<"$MODULES"

echo "==> helper CLIs (${HELPER_BINS})"
for h in $HELPER_BINS; do
  if [[ -x "$BIN/$h" ]]; then
    echo "  - $h already present"
    continue
  fi
  # authctl may ship inside auth-local release later
  if [[ "$h" == authctl && -n "$AUTH_LOCAL_TAG" ]] \
    && try_release_asset "Muxcore-Media/auth-local" "$AUTH_LOCAL_TAG" authctl authctl; then
    continue
  fi
  if copy_from_lab "$h"; then
    LAB_COPIED+=("$h")
    continue
  fi
  # Optional Go build for helpers when sources are siblings.
  if command -v go >/dev/null 2>&1; then
    case "$h" in
      authctl)
        if [[ -d "$ROOT/../auth-local/cmd/authctl" ]]; then
          echo "  - building authctl from sibling auth-local"
          (cd "$ROOT/../auth-local" && go build -o "$BIN/authctl" ./cmd/authctl)
          continue
        fi
        ;;
      gettoken)
        if [[ -d "$ROOT/../_mvp/cmd/gettoken" ]]; then
          echo "  - building gettoken from sibling _mvp"
          (cd "$ROOT/../_mvp" && go build -o "$BIN/gettoken" ./cmd/gettoken)
          continue
        fi
        ;;
    esac
  fi
  MISSING+=("$h")
  echo "  - MISSING helper $h (bootstrap-auth will need it)" >&2
done

echo "==> writing .env (TLS-off-dev defaults)"
if [[ -f "$ROOT/.env" ]]; then
  echo "    keeping existing .env (delete it to regenerate from .env.example)"
else
  cp "$ROOT/.env.example" "$ROOT/.env"
fi
# shellcheck disable=SC1091
source "$ROOT/.env"
write_view_me "$ROOT"

ESSENTIAL=(muxcored api-rest auth-local database-sqlite secrets-file admin-ui)
ESSENTIAL_OK=1
for e in "${ESSENTIAL[@]}"; do
  if [[ ! -x "$BIN/$e" ]]; then
    ESSENTIAL_OK=0
    break
  fi
done

echo
echo "======== install summary ========"
echo "root:     $ROOT"
echo "bin:      $BIN"
echo "platform: ${OS}/${ARCH}"
if ((${#LAB_COPIED[@]})); then
  echo "lab copies: ${LAB_COPIED[*]}"
  echo "  (module GitHub Releases often lack binary assets yet — lab bins are OK for laptop demo)"
fi
if ((${#MISSING[@]})); then
  echo "MISSING:  ${MISSING[*]}"
  echo
  echo "Place release binaries into bin/ or set MUXCORE_LAB_BIN to a directory of built"
  echo "module binaries (e.g. MuxCore/_mvp/bin), then re-run ./install.sh"
  echo "See README.md — Go is optional when using release/lab binaries; Docker is optional."
else
  echo "all pinned binaries present"
fi
echo
echo "Next:"
echo "  ./up.sh                 # start host stack"
echo "  ./bootstrap-auth.sh     # create admin + token"
echo "  ./smoke-fixture.sh      # offline health / fixture notes"
echo "VIEW-ME: $RUN/VIEW-ME.txt"
if [[ "$ESSENTIAL_OK" -ne 1 ]]; then
  exit 1
fi
exit 0
