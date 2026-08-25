# UI adapter: gum → dialog → whiptail → read.
# shellcheck shell=bash

UI_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UI_ROOT="${UI_ROOT:-$(cd "$UI_LIB_DIR/.." && pwd)}"
UI_BANNER="${UI_ROOT}/assets/banner.ascii"
UI_BANNER_SMALL="${UI_ROOT}/assets/banner-small.ascii"
_ONBOARD_READ_FD=0
UI_BACKEND="${UI_BACKEND:-}"

# shellcheck disable=SC1091
[[ -f "${UI_ROOT}/versions.env" ]] && source "${UI_ROOT}/versions.env"

ui_noninteractive() {
  [[ "${MUXCORE_NONINTERACTIVE:-}" == 1 ]]
}

ui_term_works() {
  command -v tput >/dev/null 2>&1 && tput cols >/dev/null 2>&1
}

ui_cols() {
  local c
  c="$(tput cols 2>/dev/null || true)"
  if [[ -n "$c" ]]; then
    printf '%s\n' "$c"
    return 0
  fi
  printf '%s\n' "${COLUMNS:-80}"
}

ui_lines() {
  local l
  l="$(tput lines 2>/dev/null || true)"
  if [[ -n "$l" ]]; then
    printf '%s\n' "$l"
    return 0
  fi
  printf '%s\n' "${LINES:-24}"
}

# SSH from Kitty/macOS often sets TERM=xterm-kitty; minimal containers lack that entry.
ui_fix_term() {
  [[ "${MUXCORE_TERM_FIXED:-}" == 1 ]] && return 0
  local orig="${TERM:-}" t
  # bash 3.2: no nameref; iterate a hard-coded list
  for t in ${MUXCORE_TERM:-} "$orig" xterm-256color xterm screen linux dumb; do
    [[ -n "$t" ]] || continue
    TERM="$t"
    export TERM
    if ui_term_works; then
      MUXCORE_TERM_FIXED=1
      export MUXCORE_TERM_FIXED
      if [[ -n "$orig" && "$t" != "$orig" ]]; then
        echo "note: using TERM=$t ($orig is not available on this host)" >&2
      fi
      return 0
    fi
  done
  export TERM=dumb
  MUXCORE_TERM_FIXED=1
  export MUXCORE_TERM_FIXED
}

onboard_require_tty() {
  if ui_noninteractive; then
    return 0
  fi
  if [[ -t 0 ]]; then
    _ONBOARD_READ_FD=0
    return 0
  fi
  if { exec </dev/tty; } 2>/dev/null; then
    _ONBOARD_READ_FD=0
    return 0
  fi
  echo "error: need a terminal. Save the script and run: bash get-muxcore.sh" >&2
  exit 1
}

ui_gum() {
  if [[ -n "${GUM_BIN:-}" && -x "$GUM_BIN" ]]; then
    "$GUM_BIN" "$@"
    return $?
  fi
  if command -v gum >/dev/null 2>&1; then
    gum "$@"
    return $?
  fi
  return 127
}

ui_gum_available() {
  ui_fix_term
  [[ "${TERM:-}" == dumb ]] && return 1
  ui_gum --version >/dev/null 2>&1
}

ui_dialog_available() {
  command -v dialog >/dev/null 2>&1
}

ui_whiptail_available() {
  command -v whiptail >/dev/null 2>&1
}

ui_detect_backend() {
  if [[ -n "$UI_BACKEND" ]]; then
    return 0
  fi
  if ui_noninteractive || [[ "${TERM:-}" == dumb ]]; then
    UI_BACKEND=plain
  elif ui_gum_available; then
    UI_BACKEND=gum
  elif ui_dialog_available; then
    UI_BACKEND=dialog
  elif ui_whiptail_available; then
    UI_BACKEND=whiptail
  else
    UI_BACKEND=plain
  fi
  export UI_BACKEND
}

ui_info() {
  ui_detect_backend
  if [[ "$UI_BACKEND" == gum ]] && ui_gum style --foreground 36 "$*" >/dev/null 2>&1; then
    ui_gum style --foreground 36 "$*"
  else
    echo "==> $*"
  fi
}

ui_ok() {
  ui_detect_backend
  if [[ "$UI_BACKEND" == gum ]] && ui_gum style --foreground 42 "$*" >/dev/null 2>&1; then
    ui_gum style --foreground 42 "$*"
  else
    echo "    $*"
  fi
}

ui_warn() {
  echo "WARN: $*" >&2
}

ui_die() {
  echo "error: $*" >&2
  exit 1
}

ui_step() {
  local n="$1" total="$2" title="$3"
  echo
  if [[ "${UI_BACKEND:-}" == gum ]] && ui_gum style --bold "Step ${n} of ${total} — ${title}" >/dev/null 2>&1; then
    ui_gum style --bold "Step ${n} of ${total} — ${title}"
  else
    printf '\n-- Step %s/%s — %s --\n' "$n" "$total" "$title"
  fi
}

ui_center_text() {
  local cols width pad line
  cols="$(ui_cols)"
  while IFS= read -r line || [[ -n "$line" ]]; do
    width="${#line}"
    if [[ "$width" -ge "$cols" ]]; then
      printf '%s\n' "$line"
      continue
    fi
    pad=$(( (cols - width) / 2 ))
    printf '%*s%s\n' "$pad" "" "$line"
  done
}

# Do not clear the screen — keeps curl|bash scrollback.
ui_splash() {
  ui_fix_term
  local cols lines art
  cols="$(ui_cols)"
  lines="$(ui_lines)"
  if [[ "$lines" -lt 12 ]]; then
    echo "MuxCore"
    echo
    return 0
  fi
  art="$UI_BANNER"
  if [[ -f "$UI_BANNER" ]]; then
    local widest=0 w
    while IFS= read -r line || [[ -n "$line" ]]; do
      w="${#line}"
      [[ "$w" -gt "$widest" ]] && widest="$w"
    done <"$UI_BANNER"
    if [[ "$widest" -gt "$cols" && -f "$UI_BANNER_SMALL" ]]; then
      art="$UI_BANNER_SMALL"
    fi
  elif [[ -f "$UI_BANNER_SMALL" ]]; then
    art="$UI_BANNER_SMALL"
  fi
  echo
  if [[ -f "$art" ]]; then
    if [[ "${UI_BACKEND:-}" == gum ]] && ui_gum style --align center --foreground 212 --bold "$(cat "$art")" >/dev/null 2>&1; then
      ui_gum style --align center --foreground 212 --bold "$(cat "$art")"
    else
      ui_center_text <"$art"
    fi
  else
    echo "MuxCore"
  fi
  echo
  echo "Personal media library setup for Linux and macOS"
  echo
}

ui_pager() {
  local file="$1"
  ui_detect_backend
  if ui_noninteractive; then
    cat "$file"
    return 0
  fi
  case "$UI_BACKEND" in
    gum)
      ui_gum pager <"$file" 2>/dev/null && return 0
      ;;
    dialog)
      dialog --textbox "$file" 22 72 && return 0
      ;;
    whiptail)
      whiptail --scrolltext --textbox "$file" 22 72 && return 0
      ;;
  esac
  if command -v less >/dev/null 2>&1; then
    less -FX "$file"
  else
    cat "$file"
  fi
}

ui_confirm() {
  local q="$1" def="${2:-false}"
  ui_detect_backend
  if ui_noninteractive; then
    [[ "$def" == true || "$def" == y || "$def" == yes ]]
    return
  fi
  case "$UI_BACKEND" in
    gum)
      if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
        ui_gum confirm --default=true "$q"
      else
        ui_gum confirm --default=false "$q"
      fi
      return
      ;;
    dialog)
      if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
        dialog --yesno "$q" 8 60
      else
        dialog --defaultno --yesno "$q" 8 60
      fi
      return
      ;;
    whiptail)
      if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
        whiptail --yesno "$q" 8 60
      else
        whiptail --defaultno --yesno "$q" 8 60
      fi
      return
      ;;
  esac
  onboard_require_tty
  local hint ans
  if [[ "$def" == true || "$def" == y || "$def" == yes ]]; then
    hint="Y/n"
  else
    hint="y/N"
  fi
  printf '%s [%s]: ' "$q" "$hint" >&2
  read -r ans || true
  ans="${ans:-$def}"
  [[ "$ans" =~ ^[Yy] ]] || [[ "$ans" == true ]]
}

# ui_choose VAR option1 option2 ... [--header TEXT]
ui_choose() {
  local var="$1"
  shift
  local header="" opts=""
  # Collect options; last --header VALUE is the header.
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == --header && $# -ge 2 ]]; then
      header="$2"
      shift 2
      continue
    fi
    opts="${opts}${opts:+
}$1"
    shift
  done
  ui_detect_backend
  local _ui_sel _ui_first
  _ui_first="$(printf '%s\n' "$opts" | head -1)"
  if ui_noninteractive; then
    printf -v "$var" '%s' "$_ui_first"
    return 0
  fi
  case "$UI_BACKEND" in
    gum)
      if [[ -n "$header" ]]; then
        _ui_sel="$(printf '%s\n' "$opts" | ui_gum choose --header "$header")" || return 1
      else
        _ui_sel="$(printf '%s\n' "$opts" | ui_gum choose)" || return 1
      fi
      printf -v "$var" '%s' "$_ui_sel"
      return 0
      ;;
    dialog|whiptail)
      local i=1 args="" line
      while IFS= read -r line; do
        [[ -n "$line" ]] || continue
        args="${args}${args:+ }${i} $(printf '%q' "$line")"
        i=$((i + 1))
      done <<EOF
$opts
EOF
      local n
      if [[ "$UI_BACKEND" == dialog ]]; then
        n="$(eval "dialog --menu $(printf '%q' "${header:-Choose}") 18 70 10 $args" 3>&1 1>&2 2>&3)" || return 1
      else
        n="$(eval "whiptail --menu $(printf '%q' "${header:-Choose}") 18 70 10 $args" 3>&1 1>&2 2>&3)" || return 1
      fi
      i=1
      while IFS= read -r line; do
        [[ -n "$line" ]] || continue
        if [[ "$n" == "$i" ]]; then
          printf -v "$var" '%s' "$line"
          return 0
        fi
        i=$((i + 1))
      done <<EOF
$opts
EOF
      printf -v "$var" '%s' "$_ui_first"
      return 0
      ;;
  esac
  onboard_require_tty
  [[ -n "$header" ]] && echo "$header" >&2
  i=1
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    echo "  $i) $line" >&2
    i=$((i + 1))
  done <<EOF
$opts
EOF
  printf 'Choose [1]: ' >&2
  read -r _ui_sel || true
  _ui_sel="${_ui_sel:-1}"
  i=1
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    if [[ "$_ui_sel" == "$i" || "$_ui_sel" == "$line" ]]; then
      printf -v "$var" '%s' "$line"
      return 0
    fi
    i=$((i + 1))
  done <<EOF
$opts
EOF
  printf -v "$var" '%s' "$_ui_first"
}

# Portable multi-select. Usage: ui_multi_list VAR preselected_csv item1 item2 ...
ui_multi_list() {
  local var="$1" pre="$2"
  shift 2
  local items="" item
  for item in "$@"; do
    items="${items}${items:+
}$item"
  done
  ui_detect_backend
  if ui_noninteractive; then
    printf -v "$var" '%s' "$pre"
    return 0
  fi
  _ui_preselected() {
    local needle="$1"
    case ",$pre," in
      *",$needle,"*) return 0 ;;
      *) return 1 ;;
    esac
  }
  case "$UI_BACKEND" in
    gum)
      local sel_flags=""
      while IFS= read -r item; do
        [[ -n "$item" ]] || continue
        if _ui_preselected "$item"; then
          sel_flags="$sel_flags --selected $(printf '%q' "$item")"
        fi
      done <<EOF
$items
EOF
      local picked
      picked="$(eval "printf '%s\\n' \"\$items\" | ui_gum choose --no-limit $sel_flags")" || return 1
      picked="$(printf '%s\n' "$picked" | paste -sd, -)"
      printf -v "$var" '%s' "$picked"
      return 0
      ;;
    dialog|whiptail)
      local args="" state i=1
      while IFS= read -r item; do
        [[ -n "$item" ]] || continue
        if _ui_preselected "$item"; then state=ON; else state=OFF; fi
        args="${args}${args:+ }$i $(printf '%q' "$item") $state"
        i=$((i + 1))
      done <<EOF
$items
EOF
      local picked_n
      if [[ "$UI_BACKEND" == dialog ]]; then
        picked_n="$(eval "dialog --checklist 'Select (space toggles)' 18 70 10 $args" 3>&1 1>&2 2>&3)" || return 1
      else
        picked_n="$(eval "whiptail --checklist 'Select (space toggles)' 18 70 10 $args" 3>&1 1>&2 2>&3)" || return 1
      fi
      local out="" n
      for n in $picked_n; do
        n="${n%\"}"
        n="${n#\"}"
        i=1
        while IFS= read -r item; do
          [[ -n "$item" ]] || continue
          if [[ "$n" == "$i" ]]; then
            out="${out}${out:+,}$item"
            break
          fi
          i=$((i + 1))
        done <<EOF
$items
EOF
      done
      printf -v "$var" '%s' "$out"
      return 0
      ;;
  esac
  onboard_require_tty
  echo "Enter numbers separated by commas (space toggles in graphical mode)." >&2
  i=1
  while IFS= read -r item; do
    [[ -n "$item" ]] || continue
    if _ui_preselected "$item"; then
      echo "  $i) $item [on]" >&2
    else
      echo "  $i) $item" >&2
    fi
    i=$((i + 1))
  done <<EOF
$items
EOF
  printf 'Selection: ' >&2
  local ans out="" n
  read -r ans || true
  if [[ -z "$ans" ]]; then
    printf -v "$var" '%s' "$pre"
    return 0
  fi
  ans="$(printf '%s' "$ans" | tr ',' ' ')"
  for n in $ans; do
    i=1
    while IFS= read -r item; do
      [[ -n "$item" ]] || continue
      if [[ "$n" == "$i" || "$n" == "$item" ]]; then
        out="${out}${out:+,}$item"
        break
      fi
      i=$((i + 1))
    done <<EOF
$items
EOF
  done
  printf -v "$var" '%s' "$out"
}

ui_input() {
  local var="$1" prompt="$2" def="${3:-}"
  local val
  ui_detect_backend
  if ui_noninteractive; then
    printf -v "$var" '%s' "$def"
    return 0
  fi
  case "$UI_BACKEND" in
    gum)
      if [[ -n "$def" ]]; then
        val="$(ui_gum input --placeholder "$def" --value "$def" --prompt "$prompt: ")" || return 1
      else
        val="$(ui_gum input --prompt "$prompt: ")" || return 1
      fi
      val="${val:-$def}"
      printf -v "$var" '%s' "$val"
      return 0
      ;;
    dialog)
      val="$(dialog --inputbox "$prompt" 8 70 "$def" 3>&1 1>&2 2>&3)" || return 1
      printf -v "$var" '%s' "${val:-$def}"
      return 0
      ;;
    whiptail)
      val="$(whiptail --inputbox "$prompt" 8 70 "$def" 3>&1 1>&2 2>&3)" || return 1
      printf -v "$var" '%s' "${val:-$def}"
      return 0
      ;;
  esac
  onboard_require_tty
  if [[ -n "$def" ]]; then
    printf '%s [%s]: ' "$prompt" "$def" >&2
  else
    printf '%s: ' "$prompt" >&2
  fi
  # bash 3.2: read -e enables readline tab-complete
  read -r -e val || true
  printf -v "$var" '%s' "${val:-$def}"
}

ui_password() {
  local var="$1" prompt="$2" def="${3:-}"
  local val
  ui_detect_backend
  if ui_noninteractive; then
    printf -v "$var" '%s' "$def"
    return 0
  fi
  case "$UI_BACKEND" in
    gum)
      val="$(ui_gum input --password --placeholder "leave blank to generate" --prompt "$prompt: ")" || return 1
      printf -v "$var" '%s' "${val:-$def}"
      return 0
      ;;
    dialog)
      val="$(dialog --insecure --passwordbox "$prompt" 8 70 3>&1 1>&2 2>&3)" || return 1
      printf -v "$var" '%s' "${val:-$def}"
      return 0
      ;;
    whiptail)
      val="$(whiptail --passwordbox "$prompt" 8 70 3>&1 1>&2 2>&3)" || return 1
      printf -v "$var" '%s' "${val:-$def}"
      return 0
      ;;
  esac
  onboard_require_tty
  printf '%s: ' "$prompt" >&2
  # shellcheck disable=SC2162
  read -s val || true
  echo >&2
  printf -v "$var" '%s' "${val:-$def}"
}

ui_spin() {
  local title="$1"
  shift
  if ui_noninteractive || [[ "${MUXCORE_DRY_RUN:-}" == 1 ]]; then
    echo "==> $title"
    "$@"
    return $?
  fi
  ui_detect_backend
  if [[ "$UI_BACKEND" == gum ]]; then
    ui_gum spin --spinner dot --title "$title" --show-output -- "$@" || {
      echo "==> $title"
      "$@"
    }
    return $?
  fi
  echo "==> $title"
  "$@"
}

ui_success() {
  local msg="$1"
  echo
  if [[ "${UI_BACKEND:-}" == gum ]] && ui_gum style --border rounded --padding "1 2" --foreground 42 "$msg" >/dev/null 2>&1; then
    ui_gum style --border rounded --padding "1 2" --foreground 42 "$msg"
  else
    echo "$msg"
  fi
  echo
}

ui_show_file() {
  ui_pager "$1"
}

ui_legal_gate() {
  local tos="${UI_ROOT}/legal/TOS.txt"
  [[ -f "$tos" ]] || ui_die "missing $tos"
  if [[ "${MUXCORE_I_AGREE:-}" == 1 ]]; then
    ui_ok "legal agreement accepted via MUXCORE_I_AGREE=1"
    return 0
  fi
  if ui_noninteractive; then
    ui_die "non-interactive install requires MUXCORE_I_AGREE=1 (the legal gate is not skipped by MUXCORE_NONINTERACTIVE alone)"
  fi
  ui_pager "$tos"
  echo
  local agreed=0
  ui_detect_backend
  if [[ "$UI_BACKEND" == gum ]]; then
    if ui_gum confirm --default=false --affirmative "I agree" --negative "I do not agree" \
      "I agree — I will only use MuxCore with media I have the right to use"; then
      agreed=1
    fi
  else
    if ui_confirm "I agree — I will only use MuxCore with media I have the right to use" false; then
      agreed=1
    fi
  fi
  if [[ "$agreed" -ne 1 ]]; then
    echo "Setup cancelled. No changes were made."
    exit 0
  fi
}

ui_record_legal() {
  local root="$1"
  local tos="${UI_ROOT}/legal/TOS.txt"
  local out="$root/data/legal-accepted.txt"
  mkdir -p "$(dirname "$out")"
  local hash tag
  tag="${INSTALLER_TAG:-unknown}"
  hash="$(sha256sum "$tos" 2>/dev/null | awk '{print $1}')"
  [[ -n "$hash" ]] || hash="$(shasum -a 256 "$tos" 2>/dev/null | awk '{print $1}')"
  cat >"$out" <<EOF
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
installer_tag=${tag}
tos_sha256=${hash:-unknown}
EOF
}
