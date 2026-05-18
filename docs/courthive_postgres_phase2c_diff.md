# Phase 2c: Compose + .env diff (review before applying)

Reviewable artifact for the cutover edits to `/opt/jim-dot-tennis/docker-compose.courthive.yml` and `/opt/jim-dot-tennis/.env`.

Captured against prod state on 2026-05-18. Re-pull and re-diff at cutover-time if more than a few weeks pass.

## Pre-edit snapshots saved on prod (rollback Lever 1)

Before applying these diffs, run:

```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cp /opt/jim-dot-tennis/docker-compose.courthive.yml /opt/jim-dot-tennis/docker-compose.courthive.yml.pre-pg && \
   cp /opt/jim-dot-tennis/.env /opt/jim-dot-tennis/.env.pre-pg && \
   ls -la /opt/jim-dot-tennis/*.pre-pg"
```

This gives Phase 3b Lever 1 something to restore from. Don't skip.

## `/opt/jim-dot-tennis/.env` — additions

Append to the end (the rest of the file stays as-is):

```diff
+
+# CourtHive Postgres credentials (added for LevelDB → Postgres migration, Phase 2c)
+# Generate a strong 32+ char password — used only inside the tennis-network,
+# never exposed to the host.
+COURTHIVE_PG_PASSWORD=<REPLACE_WITH_GENERATED_PASSWORD>
```

Generate a value with e.g. `openssl rand -base64 36 | tr -d '/+='` (gives ~48 chars of URL-safe alphabetics).

`COURTHIVE_JWT_SECRET` stays unchanged (Decision #4).

## `/opt/jim-dot-tennis/docker-compose.courthive.yml` — diff

### Change 1: Add `postgres` service

Insert this block **between the `redis` service (line 45) and the `courthive-server` service (line 47)**:

```yaml
  # Postgres for CourtHive (added Phase 2c for LevelDB → Postgres migration)
  postgres:
    image: postgres:18-alpine
    container_name: courthive-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: courthive
      POSTGRES_PASSWORD: ${COURTHIVE_PG_PASSWORD}
      POSTGRES_DB: courthive
    volumes:
      # NOTE: mount at /var/lib/postgresql, NOT /var/lib/postgresql/data.
      # postgres:18-alpine changed the on-disk layout and rejects a fresh
      # install at the older /data subpath. See migration doc Phase 0–1
      # gotcha #2.
      - courthive-pg-data:/var/lib/postgresql
    networks:
      - tennis-network
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U courthive']
      interval: 10s
      timeout: 5s
      retries: 5
```

No host port mapping — Postgres is internal-only.

### Change 2: Update `courthive-server` service

Before/after for the `courthive-server` block (lines 47–88 today):

```diff
   courthive-server:
     build:
       context: ../competition-factory-server
       dockerfile: Dockerfile
     container_name: courthive-server
     restart: unless-stopped
     depends_on:
       redis:
         condition: service_healthy
+      postgres:
+        condition: service_healthy
-    ports:
-      - "4040:4040"  # LevelDB web interface
     volumes:
       - courthive-data:/app/data      # KEEP through Phase 4 as rollback safety; remove later
       - courthive-cache:/app/cache
     environment:
       - NODE_ENV=production
-      - APP_STORAGE=fileSystem
       - APP_NAME=Competition Factory Server
       - APP_MODE=production
       - APP_PORT=8383
+      - STORAGE_PROVIDER=postgres
       - JWT_SECRET=${COURTHIVE_JWT_SECRET}
       - JWT_VALIDITY=2h
       - TRACKER_CACHE=/app/cache
       - REDIS_URL=redis://redis:6379
       - REDIS_TTL=28800000
       - REDIS_HOST=redis
       - REDIS_USERNAME=
       - REDIS_PASSWORD=
       - REDIS_PORT=6379
-      - DB_HOST=localhost
-      - DB_PORT=3838
-      - DB_USER=admin
-      - DB_PASS=adminpass
-    command: sh -c "npx net-level-server & node build/src/main.js"
+      - PG_HOST=postgres
+      - PG_PORT=5432
+      - PG_USER=courthive
+      - PG_PASSWORD=${COURTHIVE_PG_PASSWORD}
+      - PG_DATABASE=courthive
+    # `command:` override removed — the Dockerfile CMD (`node build/src/main.js`)
+    # is correct now that net-level-server is no longer needed.
     networks:
       - tennis-network
     healthcheck:
       test: ["CMD", "curl", "-f", "http://localhost:8383/"]
       interval: 30s
       timeout: 10s
       retries: 3
```

### Change 3: Add `courthive-pg-data` to the top-level `volumes:` block

Append to the `volumes:` block at the bottom of the file (after `caddy-config`):

```diff
   caddy-config:
     name: caddy-config
+  courthive-pg-data:
+    name: courthive-pg-data
```

## What this does NOT change

- The `app` (jim-dot-tennis Go app), `caddy`, and `backup` services are untouched.
- The `redis` service is untouched (CourtHive still uses Redis for caching).
- `Caddyfile.courthive` is untouched. The factory server still binds 8383 internally; Caddy still proxies the same routes.
- The `courthive-data` (LevelDB) volume stays attached to `courthive-server` through Phase 4 as a belt-and-braces against rollback. The new Postgres-backed code path won't read it.

## Validation after applying (before cutover proceeds)

```bash
# Parse-check the edited compose without bringing services up
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml config" | tail -30

# Confirm COURTHIVE_PG_PASSWORD is set (non-empty) in env
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "grep -c '^COURTHIVE_PG_PASSWORD=.\\+' /opt/jim-dot-tennis/.env"
# Expect: 1
```

Both checks must pass before any `docker compose up` runs. The `config` command catches yaml errors, missing env var references, and circular dependencies.
