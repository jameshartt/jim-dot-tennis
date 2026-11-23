# Archive

History, planning notes and the maintainer's own runbook, kept for the record. Nothing here is a guide to installing or running the software: for that see [../README.md](../README.md). Some of these files describe the maintainer's own server and older layouts that no longer exist.

## Documents

| File | What it is |
|------|------------|
| `PRODUCTION_QUICK_REFERENCE.md` | The maintainer's own runbook for their own server. Maintainer only. (A test, `TestHelpTextInOperationsDoc` in `cmd/jim-dot-tennis`, still requires its section 10 to match the command line help.) |
| `COURTHIVE_SETUP.md` | Early (pre-bundle) notes on setting up CourtHive by hand. |
| `PROFILE_IMPLEMENTATION.md` | Plan for the My Tennis profile, superseded by sprints 016 and 018. |
| `courthive_deployment_plan.md` | The original plan for adding CourtHive next to the app. |
| `courthive_postgres_migration.md` | The May 2026 move of the tournament server to Postgres. |
| `courthive_postgres_phase2c_diff.md` | The compose changes reviewed for that move. |
| `digitalocean_deployment.md` | The maintainer's source-built server set-up, replaced by `docs/install.md`. |
| `digitalocean_monitoring.md` | Monitoring notes for that server. |
| `docker_setup.md` | Early Docker set-up notes, replaced by `docs/install.md`. |
| `database_reset_guide.md` | How to reset a development database. |
| `technical_implementation_plan.md` | The original technical plan. |
| `user_experience_requirements.md` | The original UX requirements. |
| `project_overview.md` | The original project overview. |
| `automatic_nonce_extraction.md` | Notes on the old import scripts' automatic nonce extraction (the app now does this itself). |
| `TECH_DEBT_AUDIT_2026-07.md` | A July 2026 technical debt audit. |

## One-off SQL fixes (`one-off-sql/`)

Data fixes that were run once against the maintainer's database. Do not run them on another copy.

## Deployment files

Superseded files kept for history only. Do not deploy from them.

| File | What it was | Replaced by |
|------|-------------|-------------|
| `docker-compose.courthive.leveldb-era.yml` | Root `docker-compose.courthive.yml` from the pre-Postgres CourtHive stack (LevelDB storage, hardcoded database credentials, published port 4040). | `deploy/docker-compose.yml` |
| `docker-compose.courthive.postgres-phase2c.yml` | Reviewable artifact of the May 2026 CourtHive Postgres cutover (`courthive_postgres_phase2c_diff.md`). Also carried a Postgres option for the app, which is gone: the app is SQLite-only. | `deploy/docker-compose.yml` |
| `docker-compose.jim-tennis-source-built.yml`, `Caddyfile.jim-tennis-source-built` | `deploy/docker-compose.yml` and `deploy/Caddyfile` up to release 0.1.0: the layout jim.tennis still runs (CourtHive server built from a source checkout, TMX and public pages from rsynced `dist/` folders, HS256 tokens). Kept because production still matches it; see "jim.tennis until it adopts the bundle" in `PRODUCTION_QUICK_REFERENCE.md`. | `deploy/docker-compose.yml` with the `tournaments` profile |
| `Caddyfile.courthive.local-80-only` | Root `Caddyfile.courthive`: a plain `:80` site for local testing with the production block commented out. | `deploy/Caddyfile` (site address comes from `DOMAIN`) |

The packaged deployment is `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example`.
