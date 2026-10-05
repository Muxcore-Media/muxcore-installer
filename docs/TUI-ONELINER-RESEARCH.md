# MuxCore one-liner + TUI installer — architecture research

**Date:** 2026-08-24  
**Status:** Planning only. No implementation.  
**Audience:** `muxcore-installer` (laptop / homelab first-run).  
**Existing product intent:** the installer README already advertises `curl -fsSL https://getmuxcore.zem.systems | bash`, a two-step save-then-run variant, Gum as the primary UI, a legal-first walkthrough, and `MUXCORE_NONINTERACTIVE=1`. The landing/wizard scripts named there (`get-onboard.sh`, `onboard.sh`, `lib/ui.sh`, `lib/prereqs.sh`) are **not in the tree yet**. `versions.env` already pins `GUM_VERSION=v0.14.5`. `lib/common.sh` already detects `linux|darwin` × `amd64|arm64` only.

This note recommends how to implement that surface so it is safe, portable, and “graphical where possible.”

---

## Recommendation in one paragraph

Ship a **tiny static landing script** at `https://getmuxcore.zem.systems` (grey-cloud DNS → dawn/dusk nginx, same pattern as `admin`/`mux`/`auth`). That script only: detect OS/arch, download a **version-pinned installer tarball + SHA-256**, verify, extract, then exec the real wizard with stdin rebound to `/dev/tty`. The wizard is **bash + a UI adapter**: prefer a **pinned Gum binary** (PATH → cache → GitHub release with checksums), then `dialog`, then `whiptail`, then `read -e`. Do **not** make Python Textual or a custom Bubble Tea binary a hard dependency of first-run. Default process model stays **`up.sh` pid files** (already works without systemd). Offer **user systemd** then **system systemd** as opt-in. Native Windows is out of scope; **WSL is Linux**. Never `curl | sudo bash`. Never skip the legal gate unless an explicit `MUXCORE_I_AGREE=1` is set.

---

## 1. One-liner bootstrap

### What the best installers actually do

Three patterns dominate, and they are not the same:

| Pattern | Who | What the one-liner downloads | What actually installs |
|---------|-----|------------------------------|------------------------|
| **Thin landing → platform binary** | rustup (`https://sh.rustup.rs`) | POSIX `sh` script | `rustup-init` for the host triple, then that binary talks to the user |
| **Full convenience script** | Docker (`https://get.docker.com`), k3s (`https://get.k3s.io`), Homebrew (`raw.githubusercontent.com/.../install.sh`) | The entire installer | The same script (detect distro, call `apt`/`dnf`/`yum`, write systemd units) |
| **Landing → zip of a single static binary** | Bun (`https://bun.com/install`) | bash script | Latest (or tagged) `bun-$target.zip` from GitHub Releases |

**rustup** is the cleanest security split. The published script states it is “just a little script… platform detection, downloads the installer and runs it” ([rustup-init.sh](https://github.com/rust-lang/rustup/blob/master/rustup-init.sh)). Official Unix invocation is:

```text
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

([The rustup book](https://rust-lang.github.io/rustup/installation/other.html)). The landing script then:

1. Detects arch (including ELF bitness without extra tools).
2. Downloads `rustup-init` over HTTPS with `--fail --location`, TLS 1.2+, and a strong cipher list when curl/wget support it.
3. `chmod +x` and exec. If stdin is not a TTY (the `| sh` case), it rebinds the child to `/dev/tty`.
4. Does **not** SHA-256 the `rustup-init` binary in the landing script. Checksums exist as a **manual** path: `https://static.rust-lang.org/rustup/dist/{triple}/rustup-init.sha256` ([rustup book, other methods](https://rust-lang.github.io/rustup/installation/other.html)). Rust Forge is explicit that `curl | sh` exists because people want convenience, and that people who refuse the pipe should download the standalone installer and verify signatures themselves ([Rust Forge](https://rust-lang.github.io/rust-forge/infra/other-installation-methods.html)).

**Docker** documents the safer two-step as the *example*, not the pipe, and tells operators the convenience script is **not for production**:

```text
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
# optional: --dry-run
```

([Docker Engine install — Ubuntu](https://docs.docker.com/engine/install/ubuntu/)). Source lives in [docker/docker-install](https://github.com/docker/docker-install/). `get.docker.com` *is* the script (Content-Type text), not an HTML landing page.

**k3s** serves `install.sh` at `https://get.k3s.io/` (same bytes as `k3s-io/k3s/install.sh` on `main`). The script downloads `sha256sum-${ARCH}.txt` from the release and checks the binary before install. A SHA-256 of the *script itself* lives at `install.sh.sha256sum` on GitHub ([PR #8312](https://github.com/k3s-io/k3s/pull/8312)); maintainers noted the script is served off `main`, so the checksum is “pull from GitHub,” not a release artifact. That is a useful warning: **a floating landing URL and a checksum of `main` fight each other**.

**Homebrew** uses command substitution, not a pipe:

```text
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

([Homebrew/install README](https://github.com/Homebrew/install)). That still executes remote code, but the script is fetched as a string first (so bash sees a complete buffer). They require bash, refuse POSIX mode, print a plan, then `wait_for_user` (“Press RETURN/ENTER to continue”). `NONINTERACTIVE=1` skips prompts and uses `sudo -n`.

**Sandstorm** still advertises `curl https://install.sandstorm.io | bash` but documents the audit path: download `install.sh` from GitHub, optionally PGP-verify, then run ([sandstorm docs/install.md](https://github.com/sandstorm-io/sandstorm/blob/master/docs/install.md)).

### curl|bash risks (do not paper over these)

Primary technical issues, not vibes:

1. **Pipe-vs-save content split.** A server can detect `curl | bash` vs `curl -o` by watching TCP back-pressure while bash blocks on a `sleep` (or any slow command) at the top of the script. Documented in [idontplaydarts, 2016](https://www.idontplaydarts.com/2016/04/detecting-curl-pipe-bash-server-side/) and cited in [Security.SE #213401](https://security.stackexchange.com/questions/213401/is-curl-something-sudo-bash-a-reasonably-safe-installation-method). Auditors who `curl` the URL into a file may see a clean script; piped victims may receive a different body.
2. **Partial execution.** bash can start running before the transfer finishes. A mid-stream cut (or a malicious trailer after a `exit`) is not the same artifact you grepped.
3. **No integrity step.** HTTPS authenticates the *channel to whoever holds the cert*, not “this is the commit we published.” Domain expiry, DNS hijack, CDN compromise, or a stolen TLS cert all become RCE. Distro packages have a second factor (repo GPG). A one-liner does not, unless you add checksums/signatures **and** pin the hash out-of-band.
4. **`sudo` multiplies blast radius.** `curl | sudo bash` is strictly worse. Docker and Homebrew escalate only for the specific commands that need it, after a human confirmation.
5. **Typosquat / lookalike host.** `getmuxcore` vs `get-muxcore` vs a parked `.dev`. Grey-cloud `*.zem.systems` helps only if users type the real name.

Mitigations that actually work (ranked):

| Mitigation | What it buys | What it does not buy |
|------------|--------------|----------------------|
| Two-step: `curl -o` then `bash ./file` | Defeats pipe-detection; user can `less`/`sha256sum` | Still RCE if they do not look |
| `curl --proto '=https' --tlsv1.2 -fsSL` | No HTTP downgrade; fail on 404; no progress-bar junk on stderr | Not a checksum |
| Pin installer **version** in the landing script (or `MUXCORE_INSTALLER_TAG=vX`) | Reproducible; checksums can be published next to that tag | Floating `latest` undoes this |
| SHA-256 of tarball, published in the same release *and* echoed in docs / `versions.env` | Detects bit-flip / CDN swap of the *payload* | Does not help if the landing script itself is swapped |
| Cosign/GPG on the tarball (Charm already ships `checksums.txt` + sigstore for gum) | Stronger than a naked hash | Key distribution problem; most users will not verify |
| Never `curl \| sudo bash` | Limits first process to the invoking user | Wizard may still call `sudo` later, after ToS |
| `--dry-run` / print plan then confirm (Homebrew, Docker) | User sees apt/dnf/paths before they happen | Social, not cryptographic |
| Serve a **static committed file**, not a Worker that generates per-request bodies | Same bytes for `curl -o` and `curl \| bash` | Compromised origin still wins |

**Do not** put a Cloudflare Worker in front that customizes the script per User-Agent or timing. That recreates the pipe-detection primitive on your own edge.

### How to host the landing script vs the real installer

MuxCore already terminates public HTTPS on **dawn/dusk nginx**, grey-cloud DNS on **desk** (`flarectl`), origin on **vault**. That is the right place. Do not invent a second CDN story for v1.

Recommended object split:

```
https://getmuxcore.zem.systems/            → tiny landing script (text/plain, charset=utf-8)
https://getmuxcore.zem.systems/install.sh  → same file (alias, for people who want a path)
https://releases…/muxcore-installer-vX.Y.Z-{linux,darwin}-{amd64,arm64}.tar.gz
https://releases…/muxcore-installer-vX.Y.Z.sha256
```

Hosting options, in order for *this* homelab:

1. **Best: static file on vault, proxied by dawn/dusk.** Add `"getmuxcore.zem.systems"` to `nix-production/modules/roles/ingress.nix` (`proxyPass` to a tiny nginx/caddy on vault, or even `alias` a file). Grey-cloud A/AAAA on desk like `admin`/`mux`/`auth`. The landing script is a git-tracked file in `muxcore-installer` (e.g. `get-onboard.sh`) rsynced or released. **Same bytes** as GitHub raw.
2. **Also good: the landing URL is a 302 to a GitHub release asset** (`…/releases/download/vX.Y.Z/get-onboard.sh`). Pin the tag in the published one-liner docs. k3s-style “always main” is worse for MuxCore because you want a pin matrix.
3. **Acceptable: Cloudflare Worker that only `fetch()`es a pinned release URL and returns it** with `Content-Type: text/plain` and a long cache. No request-dependent body. Useful if you want `getmuxcore.zem.systems` without a new nginx vhost — but dawn/dusk already do vhosts, so this is optional.
4. **Avoid: Worker that generates the script, “latest” floating rewrite, or HTML marketing page at the same URL.** Browsers hitting `getmuxcore.zem.systems` can get a small HTML *index* at `/` **only if** `curl` still gets the script. That requires `User-Agent` sniffing, which is the same family of tricks as pipe detection. Prefer: `/` is always the script; marketing lives at `mux.zem.systems` or a `/info` page.

Publish **both** invocations in docs (Docker + MuxCore README already do this):

```text
# Convenient (accepted risk)
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash

# Auditable (recommended)
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
sha256sum -c get-muxcore.sh.sha256   # or compare to PIN-MATRIX / release notes
bash get-muxcore.sh
```

Landing-script contract (rustup + k3s hybrid):

- POSIX `sh` or bash-with-`BASH_VERSION` check (Homebrew aborts if not bash). MuxCore wizard can stay bash; landing should be the more portable of the two.
- No `sudo`. No package installs. No Gum yet.
- `set -eu`; `curl -fsSL` (or wget fallback, rustup-style; skip snap-curl).
- Detect `uname -s/-m` → `linux|darwin` × `amd64|arm64` (already in `lib/common.sh`).
- Resolve version: `MUXCORE_INSTALLER_TAG` env, else a **default tag baked into the landing file at release time** (not `latest`).
- Download tarball + `.sha256` from GitHub Releases.
- `sha256sum -c` (or `shasum -a 256` on macOS) **before** `tar xf`.
- Extract to `$XDG_CACHE_HOME/muxcore-installer/$TAG` or a temp dir; exec `onboard.sh`.
- If `[ ! -t 0 ]` and interactive: `exec … < /dev/tty` (rustup). If no `/dev/tty` and not `MUXCORE_NONINTERACTIVE=1`: fail with a clear message.
- Pass through `"$@"` so `bash get-muxcore.sh --dir ~/muxcore` works even when piped (`bash -s -- --dir …`).

---

## 2. TUI toolkit — comparison and fallback chain

### Comparison

| Toolkit | Look | Deps | Path picker | Confirm / pager | Portability | Verdict for MuxCore |
|---------|------|------|-------------|-----------------|-------------|---------------------|
| **[Gum](https://github.com/charmbracelet/gum)** | Best. Lip Gloss / Bubbles without writing Go. `choose`, `confirm`, `file`, `filter`, `input`, `pager`, `spin`, `style`, `write` | One static-ish Go binary. Official install: brew, pacman, dnf, nix, Charm apt/yum repo, or [GitHub Releases](https://github.com/charmbracelet/gum/releases) with `checksums.txt` (+ cosign/sigstore on recent tags). Windows via winget/scoop. | `gum file --directory` (tree, directories allowed) | `gum confirm` (exit 0/1); `gum pager` | linux/darwin/windows + BSD. Needs a real TTY. | **Primary UI.** Already chosen in the installer README; `GUM_VERSION` is pinned. Download the binary yourself — do not add Charm’s apt repo as a surprise. |
| **dialog** (Debian `dialog`) | 1990s ncurses boxes. Ugly but familiar on servers | Distro package (`dialog`). Not stock on macOS. | `--fselect` and `--dselect` ([Debian dialog(1)](https://manpages.debian.org/unstable/dialog/dialog.1.en.html)) | `--yesno`, `--textbox`, `--msgbox` | Excellent on Debian/Ubuntu; common on Fedora/Arch; optional elsewhere | **Best fallback** because it is the only widely packaged tool with a real file selector. |
| **whiptail** | Debian’s newt clone. Slightly cleaner than dialog | `whiptail` is on almost every Debian/Ubuntu box (often preinstalled). Fedora: `newt`. | **No `--fselect`.** Widgets: yesno, menu, inputbox, textbox, checklist, gauge ([Debian whiptail(1)](https://manpages.debian.org/unstable/whiptail/whiptail.1.en.html)) | `--yesno`, `--textbox` | Great on Debian-family; weaker elsewhere | **ToS + menus + typed paths**, not browse. |
| **fzf** | Excellent fuzzy lists | One Go binary; brew/apt/pacman/dnf/nix. [junegunn/fzf](https://github.com/junegunn/fzf) | Not a file manager. Filter a `find`/`fd` listing. Recursion of `$HOME` can be huge/slow. | Not a confirm/pager replacement | Same class as gum (need a binary) | **Optional enhancer** if already on PATH. Do not download fzf *and* gum. |
| **Gum + fallback** | Modern when possible, boxes or prompts otherwise | Gum download + tiny adapter | Per-backend | Per-backend | The only way to honor “no matter what system” | **Recommended strategy.** |
| **Python Textual** | Best-looking full-screen apps; can even serve a browser UI | Python ≥ 3.9 + `pip install textual` ([getting started](https://textual.textualize.io/getting_started/)). macOS Terminal.app is 256-color / box-drawing weak; they recommend iTerm2/Kitty/WezTerm. | You would build it | You would build it | Python is *not* guaranteed (minimal Debian, some NAS, macOS without CLT). pip on Debian is a mess (`externally-managed-environment`). | **Reject for first-run.** Beautiful, but you now have two installers (Python + MuxCore). |
| **Go Bubble Tea** | Same visual family as Gum (Gum *is* Bubble Tea widgets) | You ship **your** binary (linux/darwin × amd64/arm64). User needs nothing else. | First-class if you write it | First-class | Same publish matrix you already have for `muxcored` | **Best long-term** if the wizard grows (multi-step forms, live health). Wrong for v1: you do not yet have `get-onboard.sh`. Use Gum as the Bubble Tea you did not have to compile. Revisit when the wizard is stable. |
| **Pure bash + tput/ANSI** | Fine for banners and `read -e` | bash, `tput` (ncurses), `/dev/tty` | Readline tab-complete (`read -e -i "$default"`) | `read -r` y/N | Everywhere bash exists (Homebrew requires bash; macOS ships zsh as login but `/bin/bash` still exists, old 3.2 — **write to 3.2 or require bash 4+ and say so**) | **Last resort**, and the only one that must always work. |

### Strategy (detect, then degrade)

```
1. stdin/TTY
   - If [ -t 0 ] or /dev/tty is usable → interactive
   - Else if MUXCORE_NONINTERACTIVE=1 → flags/env only
   - Else fail: “piped without a TTY; save the script or use --yes”

2. UI backend (first hit wins)
   a. GUM already on PATH, version ≥ pin (or any 0.14+ / 2.x — decide at impl time)
   b. Cached $INSTALL_ROOT/bin/gum or $XDG_CACHE_HOME/muxcore/gum-$VER
   c. Download pinned gum tarball for this OS/arch; sha256 against published checksums.txt; install into cache
   d. command -v dialog
   e. command -v whiptail
   f. bash read -e / printf

3. Never block first-run on “please apt install gum”
   - Package-manager install of gum is optional sugar (brew/pacman/dnf/nix)
   - Adding Charm’s apt/yum repo is a trust decision; prefer the GitHub release tarball

4. TERM / color
   - If TERM=dumb or not a TTY → backend f
   - If tput colors < 8 → still allow dialog/whiptail; skip gum style chrome if it looks broken
```

**Do not** auto-install `dialog`/`whiptail` unless the user already agreed to “system prep may use sudo.” Prefer downloading gum into the *install prefix* (user-writable) so NixOS and locked-down macOS still get a TUI.

### OS notes

| OS | Reality | What the installer should do |
|----|---------|------------------------------|
| **Debian/Ubuntu** | `whiptail` often present; `dialog` one apt away; gum not in default Ubuntu repos (Charm has their own) | Use gum if we can download it; else whiptail for questions + `read -e` for paths; offer `apt-get install dialog ffmpeg` after ToS |
| **Fedora/RHEL** | `dnf install gum` works (EPEL 10 / Charm yum). `newt` provides whiptail-like tools | Same chain; `dnf` for ffmpeg |
| **Arch** | `pacman -S gum dialog fzf` | Easiest native gum |
| **NixOS** | **No `apt`/`dnf` mutation.** `nix-env -iA nixpkgs.gum` / flakes / `nix-shell -p gum ffmpeg`. Prebuilt ELF may need `patchelf` if dynamically linked ([NixOS wiki: Packaging binaries](https://wiki.nixos.org/wiki/Packaging_Binaries)). Go binaries from Charm are usually static enough to run from `$HOME`. | Detect `/etc/os-release` `ID=nixos`. **Never** `sudo apt`. Download gum into the install dir. Print a `nix-shell -p ffmpeg` / `environment.systemPackages` snippet. Do not fail the wizard because ffmpeg is missing — disable ffprobe features. |
| **macOS** | No stock dialog/whiptail. `/bin/bash` is 3.2. Homebrew is the native package manager. Gum: `brew install gum` or Darwin release tarball. | Require bash 4+ **or** stay 3.2-safe in the landing script. Prefer downloading gum over requiring Homebrew. Homebrew only if user already has it or opts in. |
| **WSL** | Linux. systemd may be present (WSL2 + `[boot] systemd=true`) or not. Paths are Linux paths; `/mnt/c/...` is a valid library root. | Treat as Linux. Warn if Windows-side Docker Desktop vs WSL docker. No `.ps1` needed. |
| **Native Windows** | rustup ships `rustup-init.exe`; Bun ships `install.ps1`; Docker is Docker Desktop. Gum exists via winget. MuxCore modules are linux/darwin only today (`detect_os_arch`). | **Out of scope.** Error with “use WSL2 or a Linux/macOS host.” Do not pretend a TUI on cmd.exe is a product. |

### Why not Textual or Bubble Tea for v1

- Textual adds a Python packaging problem that is harder than MuxCore’s own binary pin matrix.
- A custom Bubble Tea installer is the right *product* (one signed `muxcore-setup` binary, rustup-shaped) once the flow is stable. Building it now means you debug wizard copy in Go instead of bash, while `onboard.sh` does not exist yet.
- Gum **is** Bubble Tea for shell. The README already picked it. Keep that.

---

## 3. Path picker UX

Goal: default + recommended + browse, without trapping non-technical users in `find /`.

**Recommended interaction (all backends implement the same menu):**

1. Show three choices (`gum choose` / `dialog --menu` / `whiptail --menu` / numbered `read`):
   - **Recommended** — e.g. `~/muxcore` (install root) or `~/Videos/Movies` (library)
   - **Type a path** — pre-filled default, editable
   - **Browse** — only if the backend can browse
2. After any choice, **confirm the resolved absolute path** and `mkdir -p` only after the user accepts.
3. Resolve with `realpath` / `python3 -c abspath` / `~` expand — `lib/common.sh` already has `resolve_install_dir`.

**Browse implementations:**

| Backend | How | Caveats |
|---------|-----|---------|
| **Gum** | `gum file --directory "$START"` ([gum README “File”](https://github.com/charmbracelet/gum); `--directory` allows directory selection per [gum(1)](https://www.mankier.com/1/gum)) | Tree from `$START` (use `$HOME`, not `/`). Hidden files off unless `--all`. Single select only. |
| **dialog** | `--dselect /home/user/ 14 60` for directories; `--fselect` if you ever need a file ([dialog(1)](https://manpages.debian.org/unstable/dialog/dialog.1.en.html)) | Output is traditionally on stderr; use `--stdout` or `--output-fd`. Height/width must fit `tput lines/cols`. Feels dated; works on SSH. |
| **whiptail** | No file widget. Use **type a path** + optional `whiptail --menu` of `$START/*` one level at a time (home-grown pager). | Do not fake a full browser. |
| **fzf** | `find "$START" -maxdepth 3 -type d \| fzf` | Cap depth. Exclude `node_modules`, `.git`, `/proc`. Good as a *mode* under gum’s “Browse” if fzf exists and gum file feels slow. |
| **bash** | `read -e -i "$default" -p "Path: "` | Tab-complete is the 1990s UX people already know. Enable `bind 'set completion-ignore-case on'` only for that prompt if you want. |

**Defaults to offer (MuxCore-specific):**

- Install root: `~/muxcore` (user-writable; never `/opt` unless they pick it and we have sudo).
- Movies / TV / incoming: `$INSTALL/data/library`, `$INSTALL/data/library/tv`, `$INSTALL/data/downloads` as recommended; browse from `$HOME`.

Do not start a browse at `/`. Do not recurse the whole disk.

---

## 4. Dependency installation

### Detect, explain, then escalate

Copy Homebrew’s discipline ([install.sh](https://github.com/Homebrew/install/blob/HEAD/install.sh)):

- Detect tools with `command -v`.
- Detect distro with `/etc/os-release` (`ID`, `ID_LIKE`).
- `have_sudo_access`: `sudo -v` interactively, `sudo -n` when `NONINTERACTIVE`.
- `execute_sudo` only for the exact argv that needs it. Print the command first (`ohai`).
- Invalidate sudo timestamp on exit (`sudo -k`) if you created it.
- **Refuse to run the whole wizard as root** (Homebrew: “Don’t run this as root”). MuxCore should own files as the login user.

Package map (illustrative; confirm at impl time):

| Need | Debian/Ubuntu | Fedora | Arch | Alpine | macOS | NixOS |
|------|---------------|--------|------|--------|-------|-------|
| curl, git, tar | `apt-get install -y curl git tar` | `dnf install -y curl git tar` | `pacman -S --noconfirm curl git tar` | `apk add curl git tar` | CLT / brew | already or `nix-shell` |
| ffmpeg (for `media-ffprobe`) | `ffmpeg` | `ffmpeg` | `ffmpeg` | `ffmpeg` | `brew install ffmpeg` | `nix-shell -p ffmpeg` / `environment.systemPackages` |
| Docker (optional, postgres profile only) | Point at [get.docker.com two-step](https://docs.docker.com/engine/install/ubuntu/) or distro docs. **Do not pipe get.docker.com yourself as root.** | same | same | rarely | Docker Desktop / colima — opt-in | `virtualisation.docker.enable` — print snippet, do not edit `/etc/nixos` |
| gum | download tarball | `dnf install gum` *or* tarball | `pacman -S gum` *or* tarball | apk from Charm releases | brew or Darwin tarball | tarball into prefix |

**Robustness rules:**

- If a package manager is missing (NixOS, immutable OS, no sudo): **download static tools into `$INSTALL/bin`** and continue. Fail only on hard requirements (`curl`/`tar` to fetch MuxCore itself).
- Batch one sudo session for all apt/dnf packages after the user sees the list and agrees.
- Never `apt-get upgrade`. Never add third-party apt repos silently (Charm, Docker). Adding a repo is a separate confirm.
- Alpine/musl: Gum and MuxCore release binaries are typically glibc. Detect musl (`ldd --version` / `apk`) and warn or offer a supported distro. Do not claim Alpine is first-class until you publish musl builds.

### Docker

Optional. Only for `MUXCORE_PROFILE=postgres` without `DATABASE_URL` (already documented). Default sqlite path must work with **zero** containers. If they want Docker: detect `docker`/`podman`, do not install the engine unless they confirm, and prefer “here is the official two-step” over re-implementing get.docker.com.

### systemd vs user systemd vs none

k3s **requires** systemd or OpenRC and writes `/etc/systemd/system` ([k3s install.sh](https://github.com/k3s-io/k3s/blob/main/install.sh)). MuxCore should not. The laptop demo already starts binaries with `up.sh` + pid/log files.

Recommended policy:

| Mode | When | Privileges | Persist across reboot / logout |
|------|------|------------|--------------------------------|
| **A. `up.sh` (default)** | Always available | None | No. Print “run `./up.sh` after reboot” |
| **B. User systemd** | `systemctl --user` works (`[ -d /run/user/$UID/systemd ]` or `systemctl --user status` succeeds) | None to write `~/.config/systemd/user/muxcore-*.service` | Survives reboot **only if lingering** is on. `loginctl enable-linger` often needs root ([ArchWiki systemd/User](https://wiki.archlinux.org/title/Systemd/User)). Default user units die on last logout. |
| **C. System systemd** | Homelab “always on” (vault-like) | sudo to write `/etc/systemd/system` | Yes. Offer only after ToS + “this runs as $USER via `User=`” |

macOS: no systemd. Stay on `up.sh`; optional LaunchAgent later, not v1.

WSL: systemd may be off. Detect and keep A.

Never fail the install because systemd is missing. Never require linger without explaining it.

---

## 5. Legal / ToS first screen

Industry practice is **confirm-before-mutate**, not a courtroom EULA widget:

- **Homebrew:** print the plan (paths, sudo commands), then “Press RETURN/ENTER to continue or any other key to abort” (`wait_for_user` in [install.sh](https://github.com/Homebrew/install/blob/HEAD/install.sh)).
- **rustup:** interactive proceed / customize / `-y` to skip ([setup_mode.rs](https://github.com/rust-lang/rustup/blob/master/src/cli/setup_mode.rs) `--yes` / `-y`). License is the Rust/Apache/MIT stack, not a click-wrap screen.
- **Docker:** “examine scripts… before running”; `--dry-run`.
- **Commercial CLIs** (rare in this peer set) use a pager + `I agree` / `I do not agree`, default **No**.

MuxCore is different: the README already requires a **lawfully obtained media** gate before system prep. Treat that as a real click-wrap, not a RETURN-to-continue.

**Recommended pattern:**

1. **Before** gum download, sudo, or binary fetch (the landing script may fetch the *wizard tarball*; the wizard shows ToS before *product* downloads and before sudo).
2. Show the full text in a pager: `gum pager` / `dialog --textbox` / `whiptail --textbox --scrolltext` / `less`.
3. Then an explicit question: **default No**.
   - Gum: `gum confirm --default=false "I agree…"`
   - dialog/whiptail: `--yesno` with `--defaultno`
   - bash: `read` with `n` default
4. Record acceptance: timestamp, installer version, **hash of the ToS file**, username, in `$INSTALL/data/legal-accepted.txt` (or next to `.env`). Needed if you ever change the terms.
5. **Non-interactive:** do **not** skip this with only `MUXCORE_NONINTERACTIVE=1`. Require a second explicit `MUXCORE_I_AGREE=1` (or `--i-agree`). The current README line that non-interactive “skips legal gate” should be treated as a product bug for a media stack.

---

## 6. ASCII art banner

`gum style --align center --width "$cols"` is the easy path when Gum is already up ([gum style](https://github.com/charmbracelet/gum)). That happens *after* gum is on PATH, so the **first** paint (and all fallbacks) should use portable geometry:

- Width: `tput cols` if stdout is a TTY, else `${COLUMNS:-80}` ([tput / terminfo](https://linuxcommand.org/lc3_adv_tput.php)).
- Height: `tput lines`.
- Banner metrics: max visual line length (strip ANSI before measuring if the file has color), and line count (`wc -L` is GNU; on macOS compute max length in bash/awk).
- If `cols < banner_width + 2`: **do not center a clipped mess**. Fall back to a one-line wordmark (`MuxCore`) or a `--small` banner (keep two files: `banner.ascii` and `banner-small.ascii`; README already has `scripts/gen-banner.sh` + figlet).
- If `lines` is too short for banner + first dialog: skip the banner entirely.
- Center: pad each line with `(cols - width) / 2` spaces; if indent ≤ 0, print flush-left.
- Do not `clear` the screen unless you are about to run a full-screen dialog; clearing breaks `curl | bash` scrollback that lawyers and debuggers want.
- `LANG`/`LC_ALL`: figlet/ASCII should be 7-bit. Avoid box-drawing in the fallback banner (macOS Terminal.app / Linux console).

---

## Concrete architecture (v1)

```
User
  │
  ├─ docs: two-step + sha256   (preferred)
  └─ curl | bash               (supported, rustup-shaped)
          │
          ▼
  getmuxcore.zem.systems       static text/plain landing
          │  HTTPS, no sudo
          ▼
  download muxcore-installer-vX-{os}-{arch}.tar.gz
  + .sha256  → verify → extract
          │
          ▼
  onboard.sh  < /dev/tty
          │
          ├─ 1. ToS pager + confirm (default no)
          ├─ 2. Detect TTY / backend (gum → dialog → whiptail → read)
          ├─ 3. Optional: fetch gum into $PREFIX/bin (checksums)
          ├─ 4. System prep list → confirm → sudo only if needed
          │      (curl git tar; ffmpeg optional; docker optional)
          ├─ 5. Path menus (recommended / type / browse)
          ├─ 6. Fetch pinned module binaries (existing pin matrix)
          ├─ 7. Admin user, TMDB/fixture, optional Jellyfin
          └─ 8. Start via up.sh; offer user systemd; never require it
```

**UI adapter** (one function family: `ui_choose`, `ui_confirm`, `ui_input`, `ui_pager`, `ui_spin`, `ui_pick_dir`) so screens stay declarative and backends stay swappable.

**Privilege:** landing never root; wizard never requires root; sudo is per-package after a printed plan.

**Windows:** refuse with WSL2 instructions.

---

## What MuxCore already decided (do not fight it)

- One-liner host: `getmuxcore.zem.systems`.
- Safer twin: save then run.
- Gum is the intended look; version pin lives in `versions.env`.
- Legal screen is first; media is “lawfully obtained only.”
- Process supervisor today is `up.sh`, not k3s-style mandatory systemd.
- OS/arch matrix is linux/darwin × amd64/arm64.

Gaps vs this research: landing/wizard scripts are not in the repo; gum is pinned to **v0.14.5** while Charm’s current line is **v0.17 / v2** (releases publish `checksums.txt` + sigstore — worth bumping at impl time); README says non-interactive skips legal (recommend tightening).

---

## Sources

Primary / first-party:

- rustup landing script: <https://github.com/rust-lang/rustup/blob/master/rustup-init.sh>
- rustup book (curl flags, standalone + `.sha256`): <https://rust-lang.github.io/rustup/installation/other.html>
- Rust Forge (curl\|sh vs standalone signatures): <https://rust-lang.github.io/rust-forge/infra/other-installation-methods.html>
- Docker convenience script policy: <https://docs.docker.com/engine/install/ubuntu/>
- docker-install repo: <https://github.com/docker/docker-install/>
- k3s install.sh + hash download: <https://github.com/k3s-io/k3s/blob/main/install.sh>, <https://get.k3s.io/>
- k3s script checksum: <https://github.com/k3s-io/k3s/blob/main/install.sh.sha256sum>, <https://github.com/k3s-io/k3s/pull/8312>
- Homebrew installer: <https://github.com/Homebrew/install>, <https://docs.brew.sh/Installation>
- Bun install: <https://bun.sh/docs/installation>, <https://github.com/oven-sh/bun/blob/main/src/cli/install.sh>
- Sandstorm install options: <https://github.com/sandstorm-io/sandstorm/blob/master/docs/install.md>
- Charm Gum README + releases (commands, packages, checksums): <https://github.com/charmbracelet/gum>, <https://github.com/charmbracelet/gum/releases>
- Debian dialog(1) `--fselect` / `--dselect`: <https://manpages.debian.org/unstable/dialog/dialog.1.en.html>
- Debian whiptail(1) (no fselect): <https://manpages.debian.org/unstable/whiptail/whiptail.1.en.html>
- fzf: <https://github.com/junegunn/fzf>
- Textual requirements: <https://textual.textualize.io/getting_started/>
- Bubble Tea: <https://github.com/charmbracelet/bubbletea>
- systemd user + linger: <https://wiki.archlinux.org/title/Systemd/User>
- NixOS binary packaging / patchelf: <https://wiki.nixos.org/wiki/Packaging_Binaries>
- curl\|bash pipe detection: <https://www.idontplaydarts.com/2016/04/detecting-curl-pipe-bash-server-side/>
- Security.SE on `curl | sudo bash`: <https://security.stackexchange.com/questions/213401/is-curl-something-sudo-bash-a-reasonably-safe-installation-method>
- tput cols/lines: <https://linuxcommand.org/lc3_adv_tput.php>

MuxCore in-tree:

- `muxcore-installer/README.md` — advertised one-liner, Gum, legal, paths
- `muxcore-installer/versions.env` — `GUM_VERSION`, pin matrix
- `muxcore-installer/lib/common.sh` — OS/arch, path resolve
- Workspace `AGENTS.md` — `getmuxcore` host would follow dawn/dusk + desk DNS
