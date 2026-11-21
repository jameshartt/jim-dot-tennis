# Archived deployment files

Superseded files kept for history only. Do not deploy from them.

| File | What it was | Replaced by |
|------|-------------|-------------|
| `docker-compose.courthive.leveldb-era.yml` | Root `docker-compose.courthive.yml` from the pre-Postgres CourtHive stack (LevelDB storage, hardcoded database credentials, published port 4040). | `deploy/docker-compose.yml` |
| `docker-compose.courthive.postgres-phase2c.yml` | Reviewable artifact of the May 2026 CourtHive Postgres cutover (`docs/courthive_postgres_phase2c_diff.md`). Also carried a Postgres option for the app, which is gone: the app is SQLite-only. | `deploy/docker-compose.yml` |
| `Caddyfile.courthive.local-80-only` | Root `Caddyfile.courthive`: a plain `:80` site for local testing with the production block commented out. | `deploy/Caddyfile` (site address comes from `DOMAIN`) |

The reference deployment is `deploy/docker-compose.yml`, `deploy/Caddyfile` and `deploy/.env.example`.
