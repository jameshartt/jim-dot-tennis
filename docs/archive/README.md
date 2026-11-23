# Archived deployment files

Superseded files kept for history only. Do not deploy from them.

| File | What it was | Replaced by |
|------|-------------|-------------|
| `docker-compose.courthive.leveldb-era.yml` | Root `docker-compose.courthive.yml` from the pre-Postgres CourtHive stack (LevelDB storage, hardcoded database credentials, published port 4040). | `deploy/docker-compose.yml` |
| `docker-compose.courthive.postgres-phase2c.yml` | Reviewable artifact of the May 2026 CourtHive Postgres cutover (`docs/courthive_postgres_phase2c_diff.md`). Also carried a Postgres option for the app, which is gone: the app is SQLite-only. | `deploy/docker-compose.yml` |
| `docker-compose.jim-tennis-source-built.yml`, `Caddyfile.jim-tennis-source-built` | `deploy/docker-compose.yml` and `deploy/Caddyfile` up to release 0.1.0: the layout jim.tennis still runs (CourtHive server built from a source checkout, TMX and public pages from rsynced `dist/` folders, HS256 tokens). Kept because production still matches it; see "jim.tennis until it adopts the bundle" in `docs/PRODUCTION_QUICK_REFERENCE.md`. | `deploy/docker-compose.yml` with the `tournaments` profile |
| `Caddyfile.courthive.local-80-only` | Root `Caddyfile.courthive`: a plain `:80` site for local testing with the production block commented out. | `deploy/Caddyfile` (site address comes from `DOMAIN`) |

The packaged deployment is `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example`.
