# MuxCore installer

Single-machine first-run for a personal media library. MuxCore organizes and
plays files you already have. It does not download media.

`muxcore-setup` is a single self-contained Go binary with a
[Bubbletea](https://github.com/charmbracelet/bubbletea) TUI wizard — no bash
dependency chain, no `lib/*.sh` to keep in sync, nothing to `chmod +x` by
hand. It embeds every script and config template it needs (`up.sh`,
`bootstrap-auth.sh`, `smoke-fixture.sh`, policies, `.env.example`) and lays
them out in your chosen install folder on first run.

## One-line setup

Linux or macOS:

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems | bash
```

Safer two-step (same bytes — you can read the script first):

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://getmuxcore.zem.systems -o get-muxcore.sh
bash get-muxcore.sh
```

Never pipe the installer into `sudo` — it never needs root, and will refuse to run as one.

From a checkout (builds and runs `muxcore-setup` directly):

```bash
go build -o muxcore-setup ./cmd/muxcore-setup
./muxcore-setup
```

## The walkthrough

The wizard fills every screen on top; long-running steps (downloads, starting
the stack, health checks) drop a live log viewport at the bottom so you can
watch progress and any errors without losing your place.

1. **Legal agreement** — lawfully obtained media only. Scroll with the
   viewport, `y` to accept. Not skippable, even with `MUXCORE_NONINTERACTIVE`.
2. **Install folder** — default, recommended (`/opt/muxcore` if writable),
   browse, or type a path. Detects an existing install and offers
   reuse/upgrade/reinstall.
3. **Runtime** — host processes or Docker Compose (with a live check for
   Docker/Compose availability), then how it should stay running: run once,
   a user `systemd --user` service, or a system service.
4. **Libraries** — Movies, TV, Music, Books, Comics, Audiobooks (multi-select).
5. **Media folders** — pick one media root, then a single table to review and
   edit every library's path plus the watch/import folder in one place
   (catches a TV folder nested under a movie folder before it becomes a mess).
6. **Playback** — MuxCore's own player and/or an existing Jellyfin / Plex /
   Emby / DLNA server, with a live connection check (spinner) as soon as you
   enter a URL and token.
7. **Network exposure** — localhost-only (default) or all interfaces for LAN
   access, with a plain-language warning about running without TLS on a LAN.
8. **Admin account** — username + password, with reveal-on-demand (`r`) and
   a one-key strong-password generator (`ctrl+g`).
9. **Metadata** — offline fixture data or a live TMDB API key (validated
   against the TMDB API as you type), plus metadata language/region.
10. **Database** — SQLite (default, zero-config) or Postgres (paste a
    `DATABASE_URL`).
11. **Hardware transcoding** — auto-detected (VAAPI / NVENC / VideoToolbox);
    only asked when something was actually found.
12. **Port preflight** — checks the ports this install would bind and warns
    before you commit, instead of failing halfway through startup.
13. **Summary → install** — checklist of steps with a spinner per step and a
    progress bar for downloads, streaming logs live in the bottom viewport.

When finished, open **http://localhost:8082** and use the credentials in `run/VIEW-ME.txt`.

## Non-interactive / CI

```bash
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./muxcore-setup
```

`MUXCORE_NONINTERACTIVE=1` alone is **not** consent — you must also set
`MUXCORE_I_AGREE=1`. Other env vars mirror the wizard's questions:
`MUXCORE_INSTALL_DIR`, `MUXCORE_LIBRARIES` (CSV), `MUXCORE_PLAYBACK` (CSV),
`INSTALL_RUNTIME`, `MUXCORE_KEEP_MODE`, `MUXCORE_PROFILE`, `DATABASE_URL`,
`MVP_ADMIN_USER`/`MVP_ADMIN_PASSWORD`, `TMDB_API_KEY`,
`MUXCORE_METADATA_LANGUAGE`, `JELLYFIN_BASE_URL`/`JELLYFIN_API_KEY`,
`PLEX_URL`/`PLEX_TOKEN`, `EMBY_URL`/`EMBY_TOKEN`, `MUXCORE_BIND_ALL`. See
`muxcore-setup --help`.

## Prerequisites

| Tool | Required? |
|------|-----------|
| A terminal | Yes, for the interactive wizard (non-interactive mode needs none) |
| `ffmpeg` / `ffprobe` | Optional — asked if you enable the MuxCore player or video libraries |
| Docker / Podman | Optional — asked if you pick Compose or a local Postgres container |
| Go | Only to build `muxcore-setup` from source — releases ship a static binary |
| GitHub token | Only if module releases are still private (closed alpha). The installer tries public downloads first, then prompts. |

Supported OS/arch: linux/darwin × amd64/arm64. Native Windows is not supported; use WSL2.

Regenerate the ASCII splash:

```bash
./scripts/gen-banner.sh   # uses nix-shell -p figlet
```

## Manual path (skip the wizard)

Point `muxcore-setup` at an empty directory once (`MUXCORE_DRY_RUN=1` — it
extracts `up.sh`, `bootstrap-auth.sh`, `smoke-fixture.sh`, `.env`, etc. and
stops), then drive the scripts directly:

```bash
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./muxcore-setup
./up.sh
./bootstrap-auth.sh
./smoke-fixture.sh
./up.sh stop
```

## Default modules

The wizard starts a platform core plus the libraries you selected. It does **not**
install downloaders, indexers, or request/automation pipelines.

| Layer | Modules |
|-------|---------|
| Platform | `muxcored`, `api-rest`, `auth-local`, `database-sqlite` (or postgres), `secrets-file`, `encryption-aesgcm`, `call-policy-default`, `publish-policy-default`, `cache-local`, `ratelimit-tokenbucket`, `health-monitor`, `admin-ui`, `notification-default` |
| Libraries | `media-movies`, `media-tvshows`, `media-music`, `media-books`, `media-comics`, `media-audiobooks` (as selected) plus scanner, root folders, and video helpers |

Pins: [`versions.env`](versions.env). Matrix: [`PIN-MATRIX.md`](PIN-MATRIX.md).

## Layout

| Path | Role |
|------|------|
| `cmd/muxcore-setup/` | Entry point — interactive TUI (`main.go`) and env-driven non-interactive mode (`noninteractive.go`) |
| `internal/tui/` | Bubbletea model, phases (one file per wizard screen), styles, and widgets |
| `internal/wizard/` | `Answers` (collected choices) and `Pipeline` (configure → fetch → seed → start → health → admin → smoke) |
| `internal/assets/` | Embedded copies of `up.sh`, `bootstrap-auth.sh`, `smoke-fixture.sh`, `muxcore.json`, `.env.example`, `lib/`, `policies/` — the repo-root copies are symlinks into here so there is one source of truth |
| `internal/{pathutil,envfile,ghrelease,fetch,seedroots,compose,prereqs,execstream,pins,modules}/` | Ported logic from the old `lib/*.sh` (paths, `.env` read/write, GitHub release downloads, module resolution, SQLite root-folder seeding, Compose file generation, OS/hardware detection, subprocess log streaming, version pins) |
| `get-onboard.sh` | One-liner entry — downloads the pinned `muxcore-setup` binary for your OS/arch and execs it |
| `scripts/build-release.sh` | Cross-compiles `muxcore-setup` for linux/darwin × amd64/arm64 and packages the tarballs `get-onboard.sh` expects |

Developer-only: `MUXCORE_LAB_BIN` copies unpublished binaries from a local `bin/` directory. The public wizard does not mention this.

## GitHub releases

Module binaries come from `github.com/Muxcore-Media/<module>/releases`. If a
token is already in `GITHUB_TOKEN`, `GH_TOKEN`, `MUXCORE_GITHUB_TOKEN`, or
`~/.config/muxcore/github.token`, it is used. Otherwise the installer tries a
public download and prompts only if GitHub says the asset is private.

`muxcore-setup` itself is published the same way — build with
`scripts/build-release.sh` and upload with `gh release create`/`upload` (see
that script's header) — so `get-onboard.sh` has something to fetch.
