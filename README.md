# muxcore-installer

Single-machine installer for MuxCore. Downloads published `muxcored` + essential module release assets from GitHub Releases, lays out `bin/` / `data/` / `run/`, writes TLS-off-dev `.env`, and starts a **fixture-only** host stack.

No sibling monorepo is required when module release tarballs exist. Today many module tags are published without binary assets yet — on a laptop lab you can point at `_mvp/bin` as a fallback.

## Prerequisites

| Tool | Required? |
|------|-----------|
| `curl`, `tar`, `bash` | Yes |
| `gh` CLI (authenticated) | Recommended for private `Muxcore-Media` release downloads |
| Go | **Optional** if using release binaries (or lab-copied bins). Needed only to build `authctl` / `gettoken` from sibling sources when those helpers are missing. |
| Docker / Podman | **Optional**. This installer defaults to host binaries via `./up.sh`. |

Supported demo path: **fixture acquisition only** (`DOWNLOADER_ENGINE=fixture`). Live pirate indexers are not started and are refused by `smoke-fixture.sh`.

## Quick start

```bash
git clone https://github.com/Muxcore-Media/muxcore-installer.git
cd muxcore-installer
./install.sh
./up.sh
./bootstrap-auth.sh
./smoke-fixture.sh
cat run/VIEW-ME.txt
```

### Laptop lab fallback (no module release binaries yet)

If `install.sh` reports MISSING modules:

```bash
# From a MuxCore workspace that already built _mvp/bin:
MUXCORE_LAB_BIN="$HOME/Projects/MuxCore/_mvp/bin" ./install.sh
# or simply place muxcore-installer next to _mvp — install auto-detects ../_mvp/bin
```

Pins live in [`versions.env`](versions.env) (`core@v0.5.0` + MVP module tags).

## Scripts

| Script | Role |
|--------|------|
| `install.sh` | Fetch `Muxcore-Media/core@v0.5.0` + pinned module assets; lay out dirs; write `.env` + `run/VIEW-ME.txt` |
| `up.sh` / `up.sh stop` | Start/stop host processes (TLS disabled, fixture downloader, **no** pirate indexer) |
| `bootstrap-auth.sh` | Non-interactive admin user + role + session token (`authctl` / `gettoken`) |
| `smoke-fixture.sh` | Offline smoke: binary presence, core/api health, auth modules list; never hits pirate APIs |

## VIEW-ME URLs (defaults)

- Admin UI: http://localhost:8082 (`admin` / `admin-dev-only`)
- Core health: http://127.0.0.1:8080/health
- REST API: http://127.0.0.1:18080/api/v1/health
- Monitor: http://127.0.0.1:9203/status

Printed again in `run/VIEW-ME.txt` after install/up.

## Layout

```
bin/           # muxcored + modules + authctl/gettoken
data/          # sqlite, library, downloads, secrets, …
run/           # pid/log files, VIEW-ME.txt, admin.token
policies/      # bundled call/publish policy YAML for host mode
muxcore.json   # core listen config
versions.env   # pin matrix
.env.example   # TLS-off-dev + fixture defaults
```

## Notes

- `_mvp` remains the deep reference lab (full `smoke.sh`, media-ui, etc.). This repo is the end-user path.
- Do not set `PIRATEBAY_API_BASE` or `SMOKE_LIVE_ACQUISITION=1` for the supported installer demo.
- When module GoReleaser assets land, re-run `./install.sh` on a clean `bin/` to prefer release tarballs over lab copies.
