# Changelog

## [0.3.11] - 2026-10-05


### Security

- New installs run core's **household** security profile (ADR-0016 phase 1, T-M3-02g): no `MUXCORE_INSECURE_DISABLE_TLS` anywhere; `MUXCORE_PROFILE=household` in `.env`; TLS on every mesh hop with core's own CA. `up.sh` gives core `MUXCORE_DATA_DIR=mesh` (CA in `mesh/ca`, `MUXCORE_GRPC_CA_CERT_DIR`) and `MUXCORE_CA_EXPORT_DIR=mesh/public`; every module gets `MUXCORE_TLS_CA=mesh/public/ca.crt`, its own identity dir `mesh/id/<id>` (0700, `MUXCORE_TLS_DIR`) and, until it has enrolled, its single-use `MUXCORE_BOOTSTRAP_TOKEN` (ADR-0017; `mct_2_<id>_` + hex(HMAC-SHA256(secret, id)), computed with `bin/muxcored enroll token`, falling back to python3/openssl; exported, never on a command line). `MUXCORE_ENROLL_SECRET` is generated once (32 random bytes, hex) into `.env` (0600) and only core receives it. `MUXCORE_MESH_DIAL_LOCAL=true` lets peers reach host-less module addresses on loopback, which every core-issued certificate covers.
- Key material lives in `mesh/`, outside `data/`, so a backup of `data/` never contains it and Compose module containers (which mount `data/`) cannot read the CA key (ADR-0023). The installer has no backup job; the README says to never back up `mesh/`.
- Dev escape hatch: `muxcore-setup --dev` / `MUXCORE_PROFILE=dev` (writes `MUXCORE_PROFILE=dev` + the insecure flag, loud warning) and `./up.sh --dev` for one run.
- Existing installs are not switched silently: an install that runs dev (explicit, or the legacy insecure flag without a profile) stays dev on a re-run — `MUXCORE_PROFILE=dev` is pinned so ADR-0016 phase 2 keeps it working — and the installer prints the opt-in. `muxcore-setup --household` (or `MUXCORE_PROFILE=household`) stops the stack with its own `up.sh stop`, moves `bin/` to `bin.pre-household-<time>/` and the managed scripts to `*.pre-household`, installs the pinned binaries and current scripts, removes every insecure flag and the plain-HTTP `SMOKE_CORE_URL` from `.env`, and starts in household; `data/`, settings and logins are kept. Dry runs never migrate.
- Core's HTTP port is HTTPS in household: the wizard's health step, `up.sh` and `smoke-fixture.sh` verify it against `mesh/public/ca.crt`. `smoke-fixture.sh` also fails when core reports a different profile than `.env` (e.g. a stack started with `--dev`) and when a running module has no certificate in `mesh/id/`. `VIEW-ME.txt` and the done screen show the profile.
- `bootstrap-auth.sh` logs in through auth-local's `POST /login/device` (password via stdin) instead of the plaintext-only `gettoken` helper; `up.sh` (and the Compose runtime) pass `AUTH_BOOTSTRAP_USER`/`AUTH_BOOTSTRAP_PASSWORD` so auth-local creates the admin with its role on an empty database. `authctl` (fallback) verifies auth-local against core's CA; `gettoken` remains a dev-only fallback. `run/admin.token` is written 0600.
- The Compose runtime follows the profile too: core gets `mesh/ca` and `mesh/public` bind mounts, `MUXCORE_TLS_SERVER_SANS=core` and the secret from `.env`; each module gets its `mesh/id/<id>` at `/mesh/id`, the public CA at `/mesh/ca` (read-only), `MUXCORE_ENROLL_DNS_NAMES=<service>` and `MUXCORE_ENROLL_TOKEN_<ID>` from `.env`; `ADMIN_UI_INSECURE` only in dev.

### Fixed

- `up.sh` binds health-monitor's HTTP status API to `127.0.0.1:9203` by default (health-monitor ≥ v0.1.9 refuses a non-loopback bind without `HEALTH_MONITOR_HTTP_TOKEN`, so it exited on start in both profiles); admin-ui already reads it there.
- `bootstrap-auth.sh` works in the dev profile again with the pinned `authctl` (it dialed TLS because the insecure flag in `.env` was never exported).

## [0.3.10] - 2026-10-05


### Changed

- Release train train-2026.10.3 (core v0.6.15): pin matrix (`versions.env`, `PIN-MATRIX.md`) moves to core `v0.6.15` and the spool catalog `2.7.0` module tags; fallback core tag in `internal/pins/pins.go` and `lib/modules.sh` updated to `v0.6.15` (T-M2-04, FR-INS-006).

## [0.3.9] - 2026-10-05


### Changed

- Release train train-2026.10.2 (core v0.6.13): pin matrix (`versions.env`, `PIN-MATRIX.md`) moves to core `v0.6.13` and the spool catalog `2.6.0` module tags; fallback core tag in `internal/pins/pins.go` and `lib/modules.sh` updated to `v0.6.13` (T-M2-04, FR-INS-006).

## [0.3.8] - 2026-10-05


### Changed

- The database selector is now `MUXCORE_DB_BACKEND` (`sqlite|postgres`, default `sqlite`) in the wizard, `.env`, `up.sh`, `env.example`, and docs. `MUXCORE_PROFILE` is no longer used or passed to core (core v0.6.10 reserves it for the security profile `dev|household`; T-M3-02f, ADR-0016 §1). A legacy `MUXCORE_PROFILE=sqlite|postgres` is still honoured when `MUXCORE_DB_BACKEND` is unset, with a deprecation warning, and is unset before core starts; existing `.env` files are rewritten on re-run with a message.

## [0.3.7] - 2026-10-05


## [0.3.6] - 2026-10-05


### Changed

- Pin matrix (`versions.env`, `PIN-MATRIX.md`) moved to release train `train-2026.10.1`: core `v0.6.6` and all module tags matching spool catalog `2.5.0` (T-M2-04, FR-INS-006).
- Fallback core tag in `internal/pins/pins.go` and `lib/modules.sh` updated to `v0.6.6`.
