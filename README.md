# MuxCore installer

Single-machine install surface for a **first-run laptop / homelab demo**. Downloads (or copies) `muxcored` + default platform and media modules, walks you through library paths and admin login, starts a host stack, and runs a health smoke.

This is the end-user path. [`_mvp`](../_mvp) remains a developer reference lab.

## One-line setup (recommended)

Linux or macOS — paste into a terminal:

```bash
# Private org — export a GitHub PAT with repo scope first:
export GITHUB_TOKEN=ghp_...
curl -fsSL https://getmuxcore.zem.systems | bash
```

Or save and run locally (best for full interactive UI):

```bash
curl -fsSL https://getmuxcore.zem.systems -o get-onboard.sh && bash get-onboard.sh
```

From a monorepo checkout:

```bash
bash get-onboard.sh
# or
cd muxcore-installer && ./onboard.sh
```

The guided walkthrough:

1. Legal agreement (lawfully obtained media only)
2. System prep — auto-installs **curl**, **git**, **tar**, and the **Gum** UI if missing (may prompt for `sudo`)
3. Choose install folder (type, browse, or fuzzy-search paths)
4. Fetch core + default modules
5. Metadata — TMDB API key or offline demo library
6. Create your admin username/password
7. Choose movie, TV, and incoming import folders
8. Optional Jellyfin, start stack, health check

When finished, open **http://localhost:8082** and use the credentials in `run/VIEW-ME.txt`.

Set `MUXCORE_NONINTERACTIVE=1` only for automation (skips legal gate and Gum prompts).

## Prerequisites

| Tool | Required? |
|------|-----------|
| `bash`, terminal with `/dev/tty` | Yes |
| `curl`, `git`, `tar` | Yes — **installed automatically** on first run when possible |
| [Charm Gum](https://github.com/charmbracelet/gum) | Yes — **downloaded automatically** (`GUM_VERSION` in `versions.env`) |
| Go | **Optional** — only if building helper CLIs from sibling sources |
| Docker | **Optional** — offered during setup; only needed for `MUXCORE_PROFILE=postgres` |
| GitHub token | **Required** for private repos — `GITHUB_TOKEN` or `~/.config/muxcore/github.token` |
| Prebuilt module binaries | Via GitHub Releases (`github.com/Muxcore-Media/*`) **or** lab fallback (below) |

Supported OS/arch for release assets: linux/darwin × amd64/arm64.

Regenerate the ASCII splash banner:

```bash
./scripts/gen-banner.sh   # uses nix-shell -p figlet
```

## Manual path (skip the wizard)

```bash
# 1) Fetch binaries + write .env + VIEW-ME
./install.sh

# 2) Start host stack (bin/*)
./up.sh

# 3) Create admin user + session token (password printed once)
./bootstrap-auth.sh

# 4) Health smoke
./smoke-fixture.sh

# Stop
./up.sh stop
```

Or run the interactive wizard after install:

```bash
./onboard.sh
```

URLs and login defaults are written to `run/VIEW-ME.txt`.

## Default modules

Onboarding installs and starts the official **`default` + `media` spool** modules:

| Layer | Modules |
|-------|---------|
| Platform | `muxcored`, `api-rest`, `auth-local`, `database-sqlite`, `secrets-file`, `encryption-aesgcm`, `call-policy-default`, `publish-policy-default`, `cache-local`, `ratelimit-tokenbucket`, `health-monitor`, `admin-ui`, `notification-default` |
| Media | `metadata-tmdb`, `media-movies`, `media-tvshows`, `media-automation`, `media-scanner`, `media-custom-formats`, `media-rename`, `media-ffprobe`, `media-subtitles`, `media-root-folders`, `request-media`, `jellyfin` |

Pins live in [`versions.env`](versions.env). Human-readable matrix + spool sync rule: [`PIN-MATRIX.md`](PIN-MATRIX.md) (`./scripts/check-pin-matrix.sh`).

## Lab binary fallback (`MUXCORE_LAB_BIN`)

Many modules do not publish GitHub Release binary assets yet. Point the installer at a directory of built binaries (typically the laptop lab):

```bash
export MUXCORE_LAB_BIN="$HOME/Projects/MuxCore/_mvp/bin"
./install.sh
```

If `MUXCORE_LAB_BIN` is unset, `install.sh` also tries sibling `../_mvp/bin` when present.

## Scripts

| Script | Role |
|--------|------|
| `get-onboard.sh` | One-liner entry (`curl … \| bash`) — fetch installer + run wizard |
| `onboard.sh` | Interactive first-run walkthrough (Gum UI) |
| `install.sh` | Download/copy binaries, dirs, `.env`, `run/VIEW-ME.txt` |
| `up.sh` / `up.sh stop` | Start/stop host processes from `bin/` |
| `bootstrap-auth.sh` | Admin user via `authctl` + token via `gettoken` |
| `smoke-fixture.sh` | Binary gate + core/api health |
| `scripts/gen-banner.sh` | Regenerate `assets/banner.ascii` |

If `bin/muxcored` is missing, `up.sh` fails with a clear message pointing at `install.sh` / `MUXCORE_LAB_BIN`.

## Optional profile: `postgres`

Default stack uses `database-sqlite`. To opt into Postgres:

```bash
export MUXCORE_PROFILE=postgres
# Optional: point at an existing DB instead of Docker
# export DATABASE_URL=postgres://muxcore:muxcore@127.0.0.1:5432/muxcore?sslmode=disable
./up.sh
```

`up.sh` skips `database-sqlite`, starts `database-postgres` (from `bin/` when present), and if `DATABASE_URL` / `PGHOST` are unset, starts a local `postgres:16-alpine` Docker container (`muxcore-installer-pg`). Without Docker and without `DATABASE_URL`, the postgres profile fails with a clear error. See [`database-postgres/MIGRATION-SQLITE.md`](../database-postgres/MIGRATION-SQLITE.md).

## Optional: metrics / tracing (`MUXCORE_OBSERVABILITY=1`)

`health-monitor` is **always** started with the default stack. Metrics and tracing are opt-in:

```bash
export MUXCORE_OBSERVABILITY=1
./up.sh
```

Starts `metrics-prometheus` (`:9901` scrape) and `tracing-otlp` when present in `bin/` (pins in `versions.env`). Leave `OTEL_EXPORTER_OTLP_ENDPOINT` unset for slog-only spans unless a local collector is running. Combines with `MUXCORE_PROFILE=postgres`.

## Layout

```
assets/        # banner.ascii splash art
bin/           # muxcored + modules + authctl/gettoken + cached gum
cache/         # release downloads, gum binary
data/          # sqlite, auth, library, incoming imports, …
lib/           # common.sh, ui.sh, prereqs.sh, github.sh
run/           # pid/log files, VIEW-ME.txt, admin.token
policies/      # call + publish policy YAML
muxcore.json   # core config
versions.env   # release pin matrix + GUM_VERSION
```

## Dev notes

- Helper CLIs (`authctl`, `gettoken`): preferred from releases/lab; `bootstrap-auth.sh` may build from sibling `auth-local` / `_mvp` **only if present** — it does not assume a monorepo.
- Consumer media UI is off by default (`MVP_ENABLE_MEDIA_UI=0`).
