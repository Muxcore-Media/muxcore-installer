# MuxCore installer

Single-machine install surface for a **fixture-only** laptop demo. Downloads (or copies) `muxcored` + module binaries, lays out data dirs, starts a host stack, bootstraps admin auth, and runs a health smoke — **without** live pirate indexers or BitTorrent.

This is the end-user path. [`_mvp`](../_mvp) remains a developer reference lab.

## Prerequisites

| Tool | Required? |
|------|-----------|
| `curl`, `tar`, `bash` | Yes |
| Go | **Optional** — only if building helper CLIs from sibling sources |
| Docker | **Optional** — this installer runs **host binaries**, not compose |
| `gh` | Optional — helps download private release assets |
| Prebuilt module binaries | Via GitHub Releases **or** lab fallback (below) |

Supported OS/arch for release assets: linux/darwin × amd64/arm64.

## Quick start

```bash
# 1) Fetch binaries + write .env + VIEW-ME
./install.sh

# 2) Start host stack (bin/*)
./up.sh

# 3) Create admin user + session token (password printed once)
./bootstrap-auth.sh

# 4) Health smoke (core required; no pirate APIs)
./smoke-fixture.sh

# Stop
./up.sh stop
```

URLs and login defaults are written to `run/VIEW-ME.txt`.

## Lab binary fallback (`MUXCORE_LAB_BIN`)

Many modules do not publish GitHub Release binary assets yet. Point the installer at a directory of built binaries (typically the laptop lab):

```bash
export MUXCORE_LAB_BIN="$HOME/Projects/MuxCore/_mvp/bin"
./install.sh
```

If `MUXCORE_LAB_BIN` is unset, `install.sh` also tries sibling `../_mvp/bin` when present.

Pins live in [`versions.env`](versions.env).

## Fixture-only policy

Default `.env` (from `.env.example`):

- `DOWNLOADER_ENGINE=fixture`
- `TMDB_FIXTURE=1`
- `PIRATEBAY_API_BASE` / `SMOKE_LIVE_ACQUISITION` **unset**

`./up.sh` never starts `indexer-piratebay`. `./smoke-fixture.sh` requires essential binaries in `bin/`, curls core + api health, optionally bootstraps auth, and prints fixture notes — it does **not** call pirate APIs. Exit **non-zero** if binaries are missing or core/api health fails.

## Scripts

| Script | Role |
|--------|------|
| `install.sh` | Download/copy binaries, dirs, `.env`, `run/VIEW-ME.txt` |
| `up.sh` / `up.sh stop` | Start/stop host processes from `bin/` |
| `bootstrap-auth.sh` | Admin user via `authctl` + token via `gettoken` |
| `smoke-fixture.sh` | Binary gate + core/api health + fixture notes (no pirate APIs) |

If `bin/muxcored` is missing, `up.sh` fails with a clear message pointing at `install.sh` / `MUXCORE_LAB_BIN`.

## Layout

```
bin/           # muxcored + modules + authctl/gettoken
data/          # sqlite, auth, library, downloads, …
run/           # pid/log files, VIEW-ME.txt, admin.token
policies/      # call + publish policy YAML
muxcore.json   # core config
versions.env   # release pin matrix
```

## Dev notes

- Helper CLIs (`authctl`, `gettoken`): preferred from releases/lab; `bootstrap-auth.sh` may build from sibling `auth-local` / `_mvp` **only if present** — it does not assume a monorepo.
- Consumer media UI is off by default (`MVP_ENABLE_MEDIA_UI=0`).
