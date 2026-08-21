# MuxCore installer

Single-machine install surface for a **first-run laptop / homelab demo**. Downloads (or copies) `muxcored` + default platform and media modules, walks you through library paths and admin login, starts a host stack, and runs a health smoke — **without** acquisition/indexer/downloader modules.

This is the end-user path. [`_mvp`](../_mvp) remains a developer reference lab.

## One-line setup (recommended)

Linux or macOS — paste into a terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/Muxcore-Media/muxcore-installer/main/get-onboard.sh | bash
```

From a monorepo checkout:

```bash
bash get-onboard.sh
# or
cd muxcore-installer && ./get-onboard.sh
```

The guided walkthrough (7 steps):

1. Check prerequisites (curl, tar, Linux/macOS)
2. Choose install folder
3. Fetch core + default modules
4. Metadata — TMDB API key or offline demo mode
5. Create your admin username/password
6. Choose movie, TV, and incoming import folders
7. Optional Jellyfin, start stack, create login, health check

When finished, open **http://localhost:8082** and use the credentials printed in `run/VIEW-ME.txt`.

## Prerequisites

| Tool | Required? |
|------|-----------|
| `curl`, `tar`, `bash` | Yes |
| Go | **Optional** — only if building helper CLIs from sibling sources |
| Docker | **Optional** — only for `MUXCORE_PROFILE=postgres` auto-Postgres |
| `gh` | Optional — helps download private release assets |
| Prebuilt module binaries | Via GitHub Releases **or** lab fallback (below) |

Supported OS/arch for release assets: linux/darwin × amd64/arm64.

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

Acquisition/indexer/downloader modules are **not** part of this installer.

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
| `onboard.sh` | Interactive 7-step first-run walkthrough |
| `install.sh` | Download/copy binaries, dirs, `.env`, `run/VIEW-ME.txt` |
| `up.sh` / `up.sh stop` | Start/stop host processes from `bin/` |
| `bootstrap-auth.sh` | Admin user via `authctl` + token via `gettoken` |
| `smoke-fixture.sh` | Binary gate + core/api health (no acquisition modules) |

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
bin/           # muxcored + modules + authctl/gettoken
data/          # sqlite, auth, library, incoming imports, …
run/           # pid/log files, VIEW-ME.txt, admin.token
policies/      # call + publish policy YAML
muxcore.json   # core config
versions.env   # release pin matrix
```

## Dev notes

- Helper CLIs (`authctl`, `gettoken`): preferred from releases/lab; `bootstrap-auth.sh` may build from sibling `auth-local` / `_mvp` **only if present** — it does not assume a monorepo.
- Consumer media UI is off by default (`MVP_ENABLE_MEDIA_UI=0`).
