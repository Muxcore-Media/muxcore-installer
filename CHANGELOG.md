# Changelog

## [0.3.8] - 2026-10-05


### Changed

- The database selector is now `MUXCORE_DB_BACKEND` (`sqlite|postgres`, default `sqlite`) in the wizard, `.env`, `up.sh`, `env.example`, and docs. `MUXCORE_PROFILE` is no longer used or passed to core (core v0.6.10 reserves it for the security profile `dev|household`; T-M3-02f, ADR-0016 §1). A legacy `MUXCORE_PROFILE=sqlite|postgres` is still honoured when `MUXCORE_DB_BACKEND` is unset, with a deprecation warning, and is unset before core starts; existing `.env` files are rewritten on re-run with a message.

## [0.3.7] - 2026-10-05


## [0.3.6] - 2026-10-05


### Changed

- Pin matrix (`versions.env`, `PIN-MATRIX.md`) moved to release train `train-2026.10.1`: core `v0.6.6` and all module tags matching spool catalog `2.5.0` (T-M2-04, FR-INS-006).
- Fallback core tag in `internal/pins/pins.go` and `lib/modules.sh` updated to `v0.6.6`.
