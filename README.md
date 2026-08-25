# MuxCore installer

Single-machine first-run for a personal media library. MuxCore organizes and
plays files you already have. It does not download media.

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

Never pipe the installer into `sudo`.

From a checkout:

```bash
bash get-onboard.sh
# or
./onboard.sh
```

The walkthrough:

1. Legal agreement (lawfully obtained media only — not skipped by `MUXCORE_NONINTERACTIVE`)
2. System prep (curl, tar, Gum UI; optional ffmpeg / Docker)
3. Install folder, host processes vs Docker Compose
4. Libraries (movies, TV, music, books, comics, audiobooks)
5. Media folders (default, recommended, browse, or type)
6. Playback (MuxCore player and/or an existing Jellyfin / Plex / Emby / DLNA)
7. First admin login, metadata, SQLite vs Postgres
8. Fetch GitHub Release binaries, start the stack, health check

When finished, open **http://localhost:8082** and use the credentials in `run/VIEW-ME.txt`.

Non-interactive / CI:

```bash
MUXCORE_I_AGREE=1 MUXCORE_NONINTERACTIVE=1 MUXCORE_DRY_RUN=1 ./onboard.sh
```

`MUXCORE_NONINTERACTIVE=1` alone is **not** consent. You must also set `MUXCORE_I_AGREE=1`.

## Prerequisites

| Tool | Required? |
|------|-----------|
| `bash`, a terminal | Yes |
| `curl`, `tar` | Yes — installed automatically when possible |
| [Charm Gum](https://github.com/charmbracelet/gum) | Preferred UI — downloaded automatically (`GUM_VERSION` in `versions.env`). Falls back to `dialog`, `whiptail`, then prompts. |
| `ffmpeg` / `ffprobe` | Optional — asked if you enable the MuxCore player or video libraries |
| Docker / Podman | Optional — asked if you pick Compose or a local Postgres container |
| Go | No |
| GitHub token | Only if module releases are still private (closed alpha). The installer tries public downloads first, then prompts. |

Supported OS/arch: linux/darwin × amd64/arm64. Native Windows is not supported; use WSL2.

Regenerate the ASCII splash:

```bash
./scripts/gen-banner.sh   # uses nix-shell -p figlet
```

## Manual path (skip the wizard)

```bash
./install.sh
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

## Scripts

| Script | Role |
|--------|------|
| `get-onboard.sh` | One-liner entry — fetch pinned installer, rebind `/dev/tty`, run wizard |
| `onboard.sh` | Interactive first-run walkthrough |
| `install.sh` | Download selected binaries, `.env`, `VIEW-ME.txt` |
| `up.sh` / `up.sh stop` | Start/stop host processes from `bin/` |
| `bootstrap-auth.sh` | Admin user via `authctl` + token via `gettoken` |
| `smoke-fixture.sh` | Health smoke |

Developer-only: `MUXCORE_LAB_BIN` copies unpublished binaries from a local `bin/` directory. The public wizard does not mention this.

## GitHub releases

Binaries come from `github.com/Muxcore-Media/<module>/releases`. If a token is
already in `GITHUB_TOKEN`, `GH_TOKEN`, `MUXCORE_GITHUB_TOKEN`, or
`~/.config/muxcore/github.token`, it is used. Otherwise the installer tries a
public download and prompts only if GitHub says the asset is private.
