# CourtHive LevelDB → PostgreSQL Migration Plan

Status: **Phase 3 complete; only Phase 4 cleanup remains** (drafted 2026-05-10, executed end-to-end 2026-05-18)
Owner: jameshartt
Target: jim.tennis production deployment of `competition-factory-server`

## Phase 0 results (2026-05-18)

End-to-end POC against a fresh prod snapshot succeeded:

- **Snapshot**: 1.5 MB compressed, 40 files. Saved at `~/backups/courthive-leveldb-20260518-081235.tar.gz`.
- **Migration counts** (matching prod state): 1 tournament (Parks Cup 2026), 4 users, 2 providers, 1 calendar, 4 user-provider backfills. Zero access codes or reset codes.
- **Round-trip via `pg_dump --data-only`**: validated — nuke volume → re-boot → migration-runner applies 22 schemas → restore data dump → counts match exactly.
- **API smoke**: Postgres-backed server returns the migrated Parks Cup 2026 tournament + BHPLTA calendar through `/factory/tournamentinfo` and `/provider/calendar`. Login endpoint returns 401 for wrong creds (user lookup is reaching Postgres).

Four issues surfaced during Phase 0 — all worked around in the POC and now folded into the relevant phases below. See "Phase 0 gotchas" section.

## Why this is happening

Upstream `CourtHive/competition-factory-server` deleted the LevelDB storage path entirely (`src/storage/leveldb/` and `src/services/levelDB/` removed; `@gridspace/net-level-server` no longer in compose). The deployment at jim.tennis still runs on LevelDB, which means:

- Every upstream sync is now a force-keep-our-deletions exercise
- We're maintaining a code path that doesn't exist in upstream
- Eventually the leveldb storage shape will drift far enough from upstream's that the integration becomes unworkable

Migrating now (while the data is small) is the cheap moment.

## Current state on prod (re-captured 2026-05-18)

| Aspect | Value |
|---|---|
| Server | DigitalOcean 1 vCPU / **1.9 GB RAM** / 49 GB disk (already resized — see Phase 2a) |
| Memory in use | 600 MB / 1.9 GB (642 MB free, 891 MB cache, 1.3 GB available) |
| Swap | 2 GB, 0 B in use |
| Disk | 19 GB / 49 GB used (40%) — 30 GB free |
| LevelDB data size | **1.9 MB** total in `/app/data` (unchanged from 2026-05-10 capture) |
| Affected service | `courthive-server` container only (jim-dot-tennis Go app unaffected) |
| Compose file | `/opt/jim-dot-tennis/docker-compose.courthive.yml` (NOT the base `docker-compose.yml`) |
| Compose build context | `../competition-factory-server` (sibling dir, rsync target) |
| Make helpers | `make courthive-up`, `courthive-down`, `courthive-restart`, `courthive-logs` |

The 1.9 MB number is the load-bearing one — this migration is bytes, not gigabytes. It's the surrounding setup (Postgres container, schema migrations, env, rollback) that takes the time.

### Important compose detail

In the current compose, **net-level-server runs inside the `courthive-server` container** via:

```yaml
command: sh -c "npx net-level-server & node build/src/main.js"
```

This matters for the cutover: stopping `courthive-server` also kills net-level-server, so the migration script can't read LevelDB from a stopped container. See Phase 3 for the chosen workaround.

## Phase 0–1 gotchas (all worked around)

| # | Issue | Where it bites | Workaround |
|---|---|---|---|
| 1 | `pg_dump --data-only` exports 23 rows of `schema_migrations` | On restore, conflicts with rows the app's migration-runner just inserted | Add `--exclude-table=schema_migrations` to every `pg_dump` invocation |
| 2 | `postgres:18-alpine` rejects the conventional `/var/lib/postgresql/data` mount on a fresh install (upstream `docker-compose.yml` has this exact line) | Container restart-loops with "PostgreSQL data in unused mount" error | Mount the volume at `/var/lib/postgresql` instead (no `/data` suffix). Upstream compose appears to predate this image change. |
| 3 | `@gridspace/net-level@0.2.6` (the `net-level-server` bin) has a hardcoded `data/.bases` write path that ignores `DB_BASE` env | On client disconnect, the server crashes with `ENOENT: data/.bases` | Run `net-level-server` from a directory whose `./data/` IS the LevelDB dir (i.e. `cd` to the parent of the snapshot before starting it) |
| 4 | `net-level-server` always tries to bind a web UI on port 4040, regardless of need | If anything else is on 4040 (e.g. an existing local courthive-server container), a noisy `EADDRINUSE` shows in logs but the protocol port (3838) is unaffected | Harmless. Ignore unless you want to silence with `WEB_PORT=4041` |
| 5 | pnpm 11.1.2 inside `docker build` aborts the `pnpm -F audit-worker build` step with `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` | Builder stage fails before producing an image | Set `ENV CI=true` in the Dockerfile builder stage (done in the new Dockerfile commit `6eb8424`) |
| 6 | The Postgres `migration-runner.service.ts` reads SQL files at runtime from `process.cwd() + 'src/storage/postgres/migrations'` — the compiled JS in `build/` does NOT contain the migrations | Image starts but logs `Migrations directory not found: /app/src/storage/postgres/migrations`; schemas never apply | Dockerfile must `COPY --from=builder /app/src/storage/postgres/migrations ./src/storage/postgres/migrations` in the production stage. The old LevelDB-only image didn't need this because migration-runner only fires when `STORAGE_PROVIDER=postgres`. |

## What upstream provides

| Asset | Path | Notes |
|---|---|---|
| Migration script | `src/scripts/migrate-to-postgres.mjs` | Idempotent (`INSERT ... ON CONFLICT DO UPDATE`); supports `--dry-run` and `--verbose`; migrates 6 record types: `tournamentRecord`, `calendar`, `provider`, `user`, `accessCodes`, `resetCodes`. **Reads from net-level-server via its network protocol** (`netLevel.list(...)`) — net-level-server must be running when the script runs. |
| Schema migrations | `src/storage/postgres/migrations/001-*.sql` … `022-*.sql` | Applied automatically on server startup via `migration-runner.service.ts` |
| Postgres compose service | `docker-compose.yml` (upstream `main`) | `postgres:18-alpine`, default creds `courthive/courthive`, volume `courthive-pg-data`, healthcheck via `pg_isready` |
| Env template | `.env.example` (upstream `main`) | `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD`, `PG_DATABASE`. Note: `STORAGE_PROVIDER` is NOT in `.env.example` but the code reads it (`src/config/app/storage.ts`) and defaults to `leveldb` — we must set `STORAGE_PROVIDER=postgres` explicitly. |

## Decisions (locked in)

1. **Server resize 1 GB → 2 GB before adding Postgres.** **Done.** The droplet has been on 2 GB for some time — re-confirmed 2026-05-18: 1.9 GB RAM, 49 GB disk, ~1.3 GB available. Phase 2a is a no-op.
2. **Upstream feature scope after migration.** **Decided: Postgres only.** Do NOT enable the new admin-client, audit-worker, sanctioning, or provisioner modules. Smallest surface change, lowest risk, fastest to ship. Re-evaluate later if any are useful for the parks-league use case.
3. **Cutover timing.** **Decided: weekday evening, 22:00 BST**, no live tournament in flight. ~30-45 min total downtime including buffer. Specific date to be set when Phase 0 + Phase 1 are signed off.
4. **JWT_SECRET stability.** **Decided: keep current value unchanged.** Existing user sessions survive the migration — users won't have to re-log-in. The secret is already in `/opt/jim-dot-tennis/.env` as `COURTHIVE_JWT_SECRET` (mapped to `JWT_SECRET` inside the container).

---

## Phase 0 — Local proof-of-concept (no prod impact)

Goal: catch any data-shape or service-startup issues with a real copy of prod data, on local hardware where iteration is free. **This is also where we validate the dump/restore path used in Phase 3 Approach A.**

**Time estimate:** half a day

```bash
# 1. Pull prod LevelDB data to a local scratch dir
mkdir -p /tmp/courthive-leveldb-snapshot
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." \
  > /tmp/courthive-leveldb-snapshot/data.tar.gz

# 2. Set up local Postgres + net-level-server
#    Use a scratch directory with the upstream docker-compose.yml + a copy of
#    the LevelDB data restored from the snapshot above.
cd ~/Development/Tennis/competition-factory-server
git checkout -b scratch/pg-migration-poc upstream/main

# 3. Configure .env with BOTH DB_* (LevelDB source) and PG_* (Postgres target)
#    See .env.example in upstream/main, and add STORAGE_PROVIDER=leveldb
#    (we'll flip it after the migration). JWT_SECRET can be anything for POC.

# 4. Bring up Postgres (auto-applies migrations 001-022 on first server start)
docker compose up -d postgres redis
pnpm watch  # first startup applies schema migrations

# 5. Start net-level-server pointing at the restored LevelDB
#    NOTE: `pnpm hive-db` no longer exists in upstream/main (it was removed
#    along with the LevelDB storage path). Use the bundled binary directly,
#    and start it from the snapshot parent dir (Phase 0 gotcha #3):
cd /tmp/courthive-leveldb-poc    # parent of ./data/ with the extracted snapshot
DB_HOST=localhost DB_PORT=3838 DB_USER=admin DB_PASS=adminpass \
  /path/to/competition-factory-server/node_modules/.bin/net-level-server \
  > /tmp/net-level-server.log 2>&1 &

# 6. Run migration
node src/scripts/migrate-to-postgres.mjs --dry-run   # confirm record counts
node src/scripts/migrate-to-postgres.mjs --verbose   # actual run

# 7. Validate the dump/restore path (this is what we'll use in Phase 3 Approach A)
#    --exclude-table=schema_migrations is REQUIRED (Phase 0 gotcha #1)
pg_dump -h localhost -U courthive -d courthive \
  --data-only --no-owner --inserts \
  --exclude-table=schema_migrations \
  > /tmp/courthive-data-only.sql
# inspect the size and a few records — should be small (a few hundred KB)
wc -l /tmp/courthive-data-only.sql
head -50 /tmp/courthive-data-only.sql

# 8. Confirm restore works against a fresh empty Postgres (simulate prod)
docker compose down -v postgres   # nuke the volume
docker compose up -d postgres
pnpm watch                        # apply schemas again
# stop pnpm watch, then restore data-only:
psql -h localhost -U courthive -d courthive -f /tmp/courthive-data-only.sql

# 9. Switch STORAGE_PROVIDER=postgres in .env and restart pnpm watch
# 10. Point local TMX at it and walk the smoke-test checklist (see Phase 3)
```

**Gate:** if anything fails — login, tournament list, draw rendering, score save, role assignment — fix here before touching prod. Iterate as needed.

---

## Phase 1 — Rebuild `jim-tennis-deploy` from `upstream/main`

The current deploy branch is **7 commits ahead** of upstream `main`. The lowercase-email fix has already been merged upstream (as `cef5bc8`), and the pnpm pin commit is stale because upstream now declares `packageManager: pnpm@11.0.9` in `package.json` directly. Cleanest path is a clean slate.

**Strategy:** reset `jim-tennis-deploy` to `upstream/main`, then re-apply only the bits jim.tennis genuinely needs.

```bash
cd ~/Development/Tennis/competition-factory-server
git fetch upstream
git checkout jim-tennis-deploy
git tag jim-tennis-deploy-pre-pg-migration   # safety tag for rollback (see Phase 3b)
git reset --hard upstream/main
# now re-apply, as small focused commits:
```

What to re-apply:

| Item | Why kept | Source ref on old branch |
|---|---|---|
| `Dockerfile` (deploy-branch version, refreshed against upstream's build) | Upstream's compose comments out the server container — we still need our image built. Recheck `CMD` against upstream's build output path (`build/src/main.js`). | `29f212d` + `170de06` |
| `scripts/parks-cup/*` + `src/scripts/parks-cup-*.ts` | Active import pipeline, not migrated to a different location upstream | `398e716` |
| `seed-admin.js` (root) + `.gitignore` entry | Production user seeding tool; not in upstream | `38b879b` |
| `fix(storage): allow saving tournaments without parentOrganisation` | **Not in upstream.** 3-line removal in `src/storage/tournament-storage.service.ts` to unblock parks-cup imports. | `617ae05` |
| `.env.production` values for jim.tennis | URL bindings, secrets — handled outside git anyway | (live on prod) |
| `docs/parks-cup-2026-import.md` | Local record of the import pipeline | already present |

What to drop:

| Item | Why dropped |
|---|---|
| `fix(users): normalize email to lowercase` (`b0b3b4d`) | Already in `upstream/main` as `cef5bc8` |
| `chore(deploy): pin pnpm@10.27.0 and move override into pnpm-workspace.yaml` (`4f6c9f5`) | **Drop entirely.** Upstream now sets `packageManager: pnpm@11.0.9` in `package.json` and has cleaned up `pnpm-workspace.yaml`. Keeping a stale 10.27.0 pin actively breaks `corepack` behavior. |
| `fix: correct build output path in Dockerfile CMD` (`170de06`) | Will be subsumed into the new Dockerfile commit — verify the path is still `build/src/main.js` against the new build |

Push as a force-update to fork:
```bash
git push --force-with-lease origin jim-tennis-deploy
```

**Gate:** local build of the deploy-branch Dockerfile succeeds and the resulting image starts cleanly against a local Postgres (i.e. Phase 0 step 9 passes using this image).

---

## Phase 2 — Production preparation

### 2a. Server resize — done

The droplet has been on 2 GB / 49 GB for some time (re-confirmed 2026-05-18). No action needed. Leaving the original procedure below as a reference for any future resize.

> _Reference procedure (kept for the next time):_ DigitalOcean panel → droplet → Resize. Short reboot. After: verify containers via `docker compose -f docker-compose.courthive.yml ps`, check `free -h`, smoke `https://jim.tennis/` and `https://jim.tennis/api/courthive/`.

### 2b. Off-server backup of LevelDB data

Belt-and-braces backup before any production change. The DigitalOcean snapshot is one layer; this is the other.

```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." \
  > ~/backups/courthive-leveldb-$(date +%Y%m%d-%H%M%S).tar.gz
```

Keep this tarball on local disk *and* off-machine (e.g. drop a copy in Dropbox/iCloud) until Phase 4 completes.

### 2c. Update production compose file

Edit `/opt/jim-dot-tennis/docker-compose.courthive.yml`:

1. **Add a `postgres` service** mirroring upstream (with the mount-path fix from Gotcha #2):

```yaml
  postgres:
    image: postgres:18-alpine
    container_name: courthive-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${PG_USER:-courthive}
      POSTGRES_PASSWORD: ${PG_PASSWORD}
      POSTGRES_DB: ${PG_DATABASE:-courthive}
    volumes:
      - courthive-pg-data:/var/lib/postgresql   # NOT /var/lib/postgresql/data — see Phase 0 gotcha #2
    networks:
      - tennis-network
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U ${PG_USER:-courthive}']
      interval: 10s
      timeout: 5s
      retries: 5
```

(Note: do **not** expose port 5432 publicly — internal-only via `tennis-network`.)

2. **Add `courthive-pg-data` to the `volumes:` block** at the bottom of the file.

3. **Update `courthive-server`**:
   - Add `postgres: { condition: service_healthy }` under `depends_on:`
   - Add env vars: `STORAGE_PROVIDER=postgres`, `PG_HOST=postgres`, `PG_PORT=5432`, `PG_USER=courthive`, `PG_PASSWORD=${COURTHIVE_PG_PASSWORD}`, `PG_DATABASE=courthive`
   - Remove env vars: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS` (LevelDB net-level-server config — no longer used)
   - **Change `command:`** from `sh -c "npx net-level-server & node build/src/main.js"` to either remove the `command:` override entirely (lets the Dockerfile `CMD` take over) or set it explicitly to `node build/src/main.js`. The new image no longer needs net-level-server.
   - Remove the `- "4040:4040"` port mapping (LevelDB web interface — gone with leveldb)

### 2d. Update `.env` on prod

Add to `/opt/jim-dot-tennis/.env`:
```
COURTHIVE_PG_PASSWORD=<generated strong password, 32+ chars>
```

Then reference it in compose as `${COURTHIVE_PG_PASSWORD}` (see 2c above). Do **not** keep `DB_*` LevelDB vars — they're removed in 2c.

`COURTHIVE_JWT_SECRET` stays unchanged (Decision 4).

### 2e. Rsync the rebuilt deploy branch

Per Section 4 in `PRODUCTION_QUICK_REFERENCE.md`:

```bash
cd ~/Development/Tennis/competition-factory-server
rsync -avz --exclude='node_modules' --exclude='dist' --exclude='.git' --exclude='build' \
  -e "ssh -i ~/.ssh/digital_ocean_ssh" \
  ./ root@144.126.228.64:/opt/competition-factory-server/
```

Don't restart courthive-server yet — just have the rsync'd source ready for the cutover.

### 2f. DigitalOcean snapshot

Take a panel snapshot immediately before Phase 3. This is the cheapest rollback button. ~5 min to take, ~5-10 min to restore.

---

## Phase 3 — Production cutover

**Two approaches.** Approach A (local migration + dump/restore) is the chosen path because it sidesteps the net-level-server-inside-courthive-server complication and keeps prod's critical path short. Approach B (in-place with sidecar) is documented as a fallback if the dump/restore route hits an unexpected problem during Phase 0 validation.

**Announce maintenance.** Suggested message to users:
> CourtHive tournaments will be offline for ~30 min from HH:MM for a database migration. Jim.tennis league features (availability, fixtures, results) will remain available throughout.

(For an alternative "no maintenance banner" route, see Appendix A — Snapshot fallback. Not part of the default plan.)

### Approach A — Migrate locally, restore to prod (recommended)

**Why this is recommended:** the migration script (`migrate-to-postgres.mjs`) reads from net-level-server over the network. In prod, net-level-server lives inside the `courthive-server` container, so stopping that container kills its data source. Migrating locally — using the same Phase 0 environment, validated against a Phase-3-moment snapshot — avoids that entirely. The only thing pushed to prod is a tiny `data-only` SQL dump.

```bash
# Step 1: Final snapshot of prod LevelDB (after announcing maintenance)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml stop courthive-server"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." \
  > /tmp/courthive-leveldb-final-$(date +%Y%m%d-%H%M%S).tar.gz

# Step 2: Locally — restore the snapshot into the Phase 0 scratch env, run the migration
#         (Phase 0 must already be passing for this to be a sub-5-minute exercise)
cd ~/Development/Tennis/competition-factory-server
# (clear out the local LevelDB scratch dir, restore from final snapshot, start net-level-server)
# (run migration as in Phase 0 step 6)
node src/scripts/migrate-to-postgres.mjs --verbose

# Step 3: Locally — dump the migrated Postgres data
#         --exclude-table=schema_migrations is REQUIRED (Phase 0 gotcha #1):
#         without it, the dump's 23 schema_migrations rows collide with the
#         rows the app's migration-runner just inserted on prod.
pg_dump -h localhost -U courthive -d courthive \
  --data-only --no-owner --inserts \
  --exclude-table=schema_migrations \
  > /tmp/courthive-data-only-$(date +%Y%m%d-%H%M%S).sql

# Step 4: Push the dump to prod
scp -i ~/.ssh/digital_ocean_ssh /tmp/courthive-data-only-*.sql \
  root@144.126.228.64:/tmp/

# Step 5: On prod — bring up Postgres
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml up -d postgres"
# wait for healthy:
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker compose -f /opt/jim-dot-tennis/docker-compose.courthive.yml ps postgres"

# Step 6: On prod — build the new (Postgres-aware) courthive-server image
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml build courthive-server"

# Step 7: On prod — start courthive-server briefly so migration-runner applies schemas 001-022
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml up -d courthive-server && sleep 20"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker logs --tail 80 courthive-server | grep -iE '(migration|schema|applied)'"
# Expect to see migrations 001 through 022 applied. Then stop the server so the restore
# doesn't race with any startup writes:
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml stop courthive-server"

# Step 8: On prod — restore the data-only dump into the now-schema'd Postgres
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker exec -i courthive-postgres psql -U courthive -d courthive" \
  < /tmp/courthive-data-only-*.sql   # OR copy the file in first and use < inside the ssh

# (Safer alternative: scp the .sql into the container, run psql there.)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker cp /tmp/courthive-data-only-*.sql courthive-postgres:/tmp/restore.sql && \
   docker exec courthive-postgres psql -U courthive -d courthive -f /tmp/restore.sql"

# Step 9: Start courthive-server with STORAGE_PROVIDER=postgres
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml up -d courthive-server"
```

**Smoke test checklist** (run all from a real browser, not curl):

- [ ] Sign in via TMX (`https://jim.tennis/tournaments`) with a known account
- [ ] Sign in with the email in **mixed case** — confirms lowercase normalization survived
- [ ] Open an existing tournament — draws render
- [ ] Open a match — scorecard loads
- [ ] Make a trivial mutation (assign a participant, save) — confirm it persists across a refresh
- [ ] Public viewer (`https://jim.tennis/public`) loads a published tournament
- [ ] Public viewer receives a live score update (open scoring on TMX in one tab, watch public in another)
- [ ] `https://jim.tennis/api/courthive/` returns `{"message":"Factory server"}`
- [ ] Postgres data check: `docker exec courthive-postgres psql -U courthive -d courthive -c "SELECT count(*) FROM tournaments;"` matches the dry-run count from Phase 0

**Gate:** if any smoke test fails and you can't see a quick fix → **rollback** (Phase 3b).

### Approach B — In-place with net-level-server sidecar (fallback)

Use only if Approach A's dump/restore route hits an issue during Phase 0 validation.

The key trick: temporarily run net-level-server as a **separate** container that mounts the same `courthive-data` volume, so the migration script (running in a one-shot new-image container) can read from it.

```bash
# Step 1: Tag the old image for safe rollback (see Phase 3b)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker tag jim-dot-tennis_courthive-server:latest jim-dot-tennis_courthive-server:pre-pg-migration"

# Step 2: Stop the old courthive-server (releases the LevelDB lock)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml stop courthive-server"

# Step 3: Start a temporary net-level-server sidecar from the OLD image, command override
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run -d --rm --name courthive-leveldb-sidecar \
     --network jim-dot-tennis_tennis-network \
     -v courthive-data:/app/data \
     -e DB_HOST=0.0.0.0 -e DB_PORT=3838 -e DB_USER=admin -e DB_PASS=adminpass \
     jim-dot-tennis_courthive-server:pre-pg-migration \
     sh -c 'cd /app && npx net-level-server'"
# verify it's serving:
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker logs courthive-leveldb-sidecar | tail"

# Step 4: Bring up Postgres + schema-prime via courthive-server (as Approach A Steps 5-7)

# Step 5: Run the migration script as a one-shot container of the NEW image,
#         pointed at the sidecar for the LevelDB side and prod Postgres for the target
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run --rm --network jim-dot-tennis_tennis-network \
     -e DB_HOST=courthive-leveldb-sidecar -e DB_PORT=3838 -e DB_USER=admin -e DB_PASS=adminpass \
     -e PG_HOST=postgres -e PG_PORT=5432 -e PG_USER=courthive \
     -e PG_PASSWORD=\$(grep ^COURTHIVE_PG_PASSWORD /opt/jim-dot-tennis/.env | cut -d= -f2) \
     -e PG_DATABASE=courthive \
     jim-dot-tennis_courthive-server:latest \
     node src/scripts/migrate-to-postgres.mjs --verbose"

# Step 6: Stop the sidecar
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker stop courthive-leveldb-sidecar"

# Step 7: Start courthive-server with STORAGE_PROVIDER=postgres (as Approach A Step 9)
```

Run the same smoke-test checklist as Approach A.

### Phase 3b — Rollback (only if smoke test fails)

Three rollback levers, in order of preference. The first two require the deploy branch's pre-migration state — we tagged it as `jim-tennis-deploy-pre-pg-migration` in Phase 1.

**Lever 1: Re-deploy the pre-migration image and flip env back to LevelDB** (~5 min)

```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 << 'EOF'
cd /opt/jim-dot-tennis
docker compose -f docker-compose.courthive.yml stop courthive-server
# Revert compose changes (the diff from 2c) — easiest is a saved-pre-edit copy:
cp docker-compose.courthive.yml.pre-pg docker-compose.courthive.yml
# Revert .env (drop PG_*, restore DB_* if removed — keep a pre-edit copy too)
cp .env.pre-pg .env
# Retag the saved old image as latest and bring back up
docker tag jim-dot-tennis_courthive-server:pre-pg-migration jim-dot-tennis_courthive-server:latest
docker compose -f docker-compose.courthive.yml up -d courthive-server
EOF
```

**Before starting Phase 3, save the pre-edit copies:**
```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cp /opt/jim-dot-tennis/docker-compose.courthive.yml /opt/jim-dot-tennis/docker-compose.courthive.yml.pre-pg && \
   cp /opt/jim-dot-tennis/.env /opt/jim-dot-tennis/.env.pre-pg"
```

**Lever 2: Rebuild the courthive-server image from the safety git tag** (~10-15 min)

If Lever 1 fails (e.g. the saved image is gone), rebuild from the tagged source:

```bash
# locally
cd ~/Development/Tennis/competition-factory-server
git checkout jim-tennis-deploy-pre-pg-migration
rsync -avz --exclude='node_modules' --exclude='dist' --exclude='.git' --exclude='build' \
  -e "ssh -i ~/.ssh/digital_ocean_ssh" \
  ./ root@144.126.228.64:/opt/competition-factory-server/
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose -f docker-compose.courthive.yml build courthive-server && \
   docker compose -f docker-compose.courthive.yml up -d courthive-server"
# Don't forget to also revert compose + env per Lever 1.
```

**Lever 3: DigitalOcean snapshot restore** (~5-10 min downtime, full system rewind)

Last resort if image-level rollback fails. Restores the entire droplet to the pre-migration snapshot taken in Phase 2f. Everything on the droplet — not just CourtHive — reverts; any jim-dot-tennis work since the snapshot is lost. Coordinate with the user before pulling this lever.

After any rollback: post-mortem before another attempt — what failed, why, and what the next Phase 0 iteration must catch.

---

## Phase 4 — Cleanup (1-2 weeks after stable run)

Wait for at least one league night + a tournament weekend before declaring the migration stable. Then:

- [ ] Remove `command:` net-level-server reference from `docker-compose.courthive.yml` (already done in 2c, but double-check)
- [ ] Drop the LevelDB Docker volume: `docker volume rm jim-dot-tennis_courthive-data` (use `docker volume ls` to confirm the exact name; the prefix follows the compose project name)
- [ ] Delete the off-server LevelDB tarball backups older than 30 days (keep one as a long-term archive)
- [ ] Remove the safety git tag once confidence is established: `git tag -d jim-tennis-deploy-pre-pg-migration` (and `--delete` on origin)
- [ ] Remove the saved compose/env pre-edit copies: `rm /opt/jim-dot-tennis/docker-compose.courthive.yml.pre-pg /opt/jim-dot-tennis/.env.pre-pg`
- [ ] Update `PRODUCTION_QUICK_REFERENCE.md`:
  - Section 8 (Create CourtHive User): rewrite to use `psql` against the `users` table instead of net-level-client. The simpler form is: hash the password locally with bcrypt and `INSERT INTO users ...` — or use `src/scripts/admin-user.mjs --storage postgres` if it covers the use case.
  - Section 8b (Reset Password): `UPDATE users SET password = $1 WHERE email = $2` after a `bcrypt.hash()` step.
  - Section 4: remove any pnpm-pin notes (pin no longer applied per Phase 1).
- [ ] Establish a new backup baseline: nightly `pg_dump` to off-server storage. Add to whatever cron/Makefile target currently handles tennis-db backups.

---

## Risks and mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Migration script bug — record shape mismatch | Medium | Phase 0 local POC catches this against a real data copy before prod |
| Memory pressure during cutover | Low (post-resize) | Resize to 2 GB done in Phase 2a |
| Smoke test reveals a missing upstream feature we depend on | Low-medium | Phase 3b Lever 1 (5-min image rollback) |
| User session invalidation | Low (controllable) | Keep `COURTHIVE_JWT_SECRET` stable across cutover — existing tokens stay valid (Decision 4) |
| Schema migrations fail on startup | Low | Run them in Phase 0 against a copy first; Phase 3 Step 7 grep'd for migration log lines before restoring data |
| Disk space (currently 79% full) | Low | 1.9 MB → low double-digit MB in Postgres + container overhead. Watch but unlikely to bite |
| net-level-server in old container dies before migration reads it | Originally Medium → now N/A | Approach A migrates locally against a final snapshot — prod never needs net-level-server during cutover. Approach B sidecar covers the fallback. |
| Pnpm version mismatch breaks image build | Low | Dropping the stale 10.27.0 pin in Phase 1 lets upstream's `packageManager: pnpm@11.0.9` take over; Phase 1 gate is local build success |
| Restore dump out of sync with prod (writes after snapshot) | Low | Phase 3 Step 1 stops courthive-server *before* taking the final snapshot — no writes possible during the migration window |

## Reference: where the data lives during each phase

| Phase | Source of truth | Notes |
|---|---|---|
| Pre-migration | LevelDB at `/app/data` in `courthive-server` container | net-level-server speaks the protocol |
| Phase 0 (local POC) | LevelDB snapshot → local Postgres | Prod untouched |
| Phase 3 Approach A | LevelDB (frozen final snapshot) → local Postgres → prod Postgres via `pg_dump --data-only` | Prod LevelDB volume stays read-only; data moves through a developer machine |
| Phase 3 Approach B | LevelDB on prod (via sidecar) → prod Postgres | Migration runs entirely on prod |
| Post-migration (Phase 3 complete) | Postgres in `courthive-postgres` container, volume `courthive-pg-data` | LevelDB volume retained but inert until Phase 4 |
| After Phase 4 | Postgres only | LevelDB volume removed |

## Open work to do before scheduling Phase 3

- [x] **Phase 0 local POC** — done 2026-05-18. Migration + dump/restore round-trip validated against a fresh prod snapshot; API smoke passed. Four gotchas surfaced and folded into the relevant phases above.
- [x] **Phase 1 deploy-branch rebuild** — done 2026-05-18. Safety tag `jim-tennis-deploy-pre-pg-migration` at `4f6c9f5` (pushed to origin). New `jim-tennis-deploy` HEAD `54e588c` = `upstream/main` (`13606b1`) + 5 commits: gitignore-seed-admin, parks-cup pipeline, parentOrganisation fix, Dockerfile, migrations-COPY fix. Local build + boot against pgpoc Postgres verified; image serves Parks Cup 2026 via the API.
- [x] **Phase 2a server resize** — already done before this migration started (droplet has been on 2 GB / 49 GB for some time). No action needed.
- [x] **Phase 2c compose diff prepared** — done 2026-05-18. Diffs in `courthive_postgres_phase2c_diff.md`; the validated full target compose is `courthive_postgres_phase2c.docker-compose.courthive.yml` (parses cleanly via `docker compose config`).
- [x] **Phase 3 cutover** — executed 2026-05-18, ~20 min window (10:13:52 → 10:33 BST). Approach A (local migrate + dump/restore) ran clean: 1 tournament + 4 users + 2 providers + 1 calendar + 4 user-provider backfills landed in prod Postgres. Initial smoke test green. All three forks deployed in the same window (factory-server new image + TMX/courthive-public fresh dist/ rsynced).
- [ ] **Phase 4 cleanup** (wait 1–2 weeks): remove `courthive-data` LevelDB volume, delete `jim-tennis-deploy-pre-pg-migration` tag, remove `.pre-pg` files from prod, retire `jim-dot-tennis-courthive-server:pre-pg-migration` image tag, update `PRODUCTION_QUICK_REFERENCE.md` to use psql for admin user CRUD, baseline a pg_dump backup cadence.

---

## Appendix A — Snapshot fallback for transparent downtime (optional, not in default plan)

The default plan accepts ~30 min of advertised downtime, which is appropriate for a weekday-evening cutover with no live tournament in flight. If a future migration needs to happen *during* a live tournament weekend — or if we just want to suppress the maintenance banner — the public viewer can be made to serve a frozen-but-correct snapshot of the active tournament(s) during the window.

**How it works**

The public viewer (`courthive-public`) reads from `courthive-server` via a small set of POST endpoints (`/factory/tournamentinfo`, `/factory/eventdata`, `/factory/scheduledmatchUps`, `/factory/participants`, `/provider/calendar`) plus a Socket.IO connection for live ticks. During the cutover, if those endpoints are routed to a tiny "snapshot fallback" service that replays pre-recorded responses, the viewer sees a static-but-accurate tournament and refreshes/new tabs keep working. Socket.IO connections fail silently — viewers don't see live ticks during the window, which is acceptable because no scoring is happening anyway.

**Shape of the implementation** (estimated 4-6 hours of careful work)

1. **Snapshot script** — small Node script that hits each public-viewer endpoint for the active tournament(s) and writes `{request-body-hash → response-json}` to a JSON file on disk.
2. **Fallback service** — ~50 LOC Express server (or equivalent) that loads the snapshot file and matches incoming POST requests by hashing the body, returning the pre-recorded response. Returns a 503 if the body doesn't hash to anything in the snapshot.
3. **Caddy upstream failover** — configure `Caddyfile.courthive` so `/api/courthive/*` has `courthive-server` as primary and the fallback service as a failover target with a short passive health-check (`fail_duration 5s`, `max_fails 3`). When the real server goes down during cutover, Caddy routes to the snapshot automatically; when it comes back, traffic flips forward.
4. **Pre-cutover**: run the snapshot script against current prod, deploy the fallback service.
5. **Cutover**: proceed with Phase 3 normally. The fallback handles traffic while courthive-server is down.
6. **Post-cutover**: leave the fallback in place for a few hours as a belt-and-braces against any startup hiccups, then remove.

**Why this is documented but not in the default plan**

- The current cutover window targets low-activity hours with no live tournament. A maintenance banner during that window costs essentially nothing.
- The fallback hides downtime from *passive viewers only*. Anyone actively scoring on TMX during the window still hits a brick wall — we need to schedule away from live tournaments regardless.
- Build/test/deploy/teardown of the fallback is a non-trivial fraction of the total migration effort. For a 30-min window, the ROI is low.
- Worth building if the next migration is bigger (e.g. schema-shape change requiring longer downtime) or has to happen during a live event.

Keep this appendix for the next migration that actually needs it.
