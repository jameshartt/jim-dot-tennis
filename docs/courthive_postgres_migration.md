# CourtHive LevelDB → PostgreSQL Migration Plan

Status: **Drafted, not yet executed** (drafted 2026-05-10)
Owner: jameshartt
Target: jim.tennis production deployment of `competition-factory-server`

## Why this is happening

Upstream `CourtHive/competition-factory-server` deleted the LevelDB storage path entirely (`src/storage/leveldb/` and `src/services/levelDB/` removed; `@gridspace/net-level-server` no longer in compose). The deployment at jim.tennis still runs on LevelDB, which means:

- Every upstream sync is now a force-keep-our-deletions exercise
- We're maintaining a code path that doesn't exist in upstream
- Eventually the leveldb storage shape will drift far enough from upstream's that the integration becomes unworkable

Migrating now (while the data is small) is the cheap moment.

## Current state on prod (captured 2026-05-10)

| Aspect | Value |
|---|---|
| Server | DigitalOcean 1 vCPU / 961 MB RAM / 25 GB disk |
| Memory in use | 615 MB / 961 MB (77 MB free, 458 MB cache, 346 MB available) |
| Swap | 2 GB, 236 MB in use |
| Disk | 19 GB / 25 GB used (79%) — 5.1 GB free |
| LevelDB data size | **1.9 MB** total in `/app/data` |
| Affected service | `courthive-server` container only (jim-dot-tennis Go app unaffected) |
| Compose context | `/opt/competition-factory-server/` (sibling to `/opt/jim-dot-tennis/`) |

The 1.9 MB number is the load-bearing one — this migration is bytes, not gigabytes. It's the surrounding setup (Postgres container, schema migrations, env, rollback) that takes the time.

## What upstream provides

| Asset | Path | Notes |
|---|---|---|
| Migration script | `src/scripts/migrate-to-postgres.mjs` | Idempotent (`INSERT ... ON CONFLICT DO UPDATE`); supports `--dry-run` and `--verbose`; migrates 6 record types: `tournamentRecord`, `calendar`, `provider`, `user`, `accessCodes`, `resetCodes` |
| Schema migrations | `src/storage/postgres/migrations/001-*.sql` … `022-*.sql` | Applied automatically on server startup via `migration-runner.service.ts` |
| Postgres compose service | `docker-compose.yml` (upstream `main`) | `postgres:18-alpine`, default creds `courthive/courthive`, volume `courthive-pg-data`, healthcheck via `pg_isready` |
| Env template | `.env.example` (upstream `main`) | `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD`, `PG_DATABASE` |

## Decisions to lock in before starting

1. **Server resize 1 GB → 2 GB before adding Postgres?**
   - Recommendation: **yes**, ~$6/month delta. Postgres baseline (~80-150 MB) on a box already at 615/961 MB is workable via swap but leaves zero headroom for traffic spikes or backups. Resize is ~30 min one-time downtime.
2. **Upstream feature scope after migration?**
   - Recommendation: **Postgres only**. Don't enable the new admin-client / audit-worker / sanctioning / provisioner modules. Smallest surface change, lowest risk, fastest to ship. Re-evaluate later if any are useful for the parks league use case.
3. **Cutover timing.**
   - Suggested window: weekday evening, ~22:00 BST. Low tennis activity. ~30-45 min downtime including buffer.
4. **JWT_SECRET stability.**
   - Keep current `JWT_SECRET` in `.env` unchanged so existing user sessions survive the migration. Users won't have to re-log-in.

---

## Phase 0 — Local proof-of-concept (no prod impact)

Goal: catch any data-shape or service-startup issues with a real copy of prod data, on local hardware where iteration is free.

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
# (work on a scratch branch off upstream/main — see Phase 1 for the deploy branch)
git checkout -b scratch/pg-migration-poc upstream/main

# 3. Configure .env with BOTH DB_* (LevelDB source) and PG_* (Postgres target)
#    See .env.example in upstream/main.

# 4. Run the schema (auto-applied by migration-runner on first server start)
docker compose up -d postgres redis
pnpm watch  # first startup applies migrations 001-022

# 5. Start net-level-server pointing at the restored LevelDB
pnpm hive-db

# 6. Run migration
node src/scripts/migrate-to-postgres.mjs --dry-run   # confirm record counts
node src/scripts/migrate-to-postgres.mjs --verbose   # actual run

# 7. Switch CFS to STORAGE_PROVIDER=postgres and restart pnpm watch
# 8. Point local TMX at it and walk the smoke-test checklist (see Phase 3)
```

**Gate:** if anything fails — login, tournament list, draw rendering, score save, role assignment — fix here before touching prod. Iterate as needed.

---

## Phase 1 — Rebuild `jim-tennis-deploy` from `upstream/main`

The current deploy branch is 4 commits ahead of upstream `main`, but most of that is now redundant (lowercase email fix already merged upstream) or about to be (Dockerfile and pnpm pinning may need to change for the Postgres-era setup anyway). Cleanest path is a clean slate.

**Strategy:** reset `jim-tennis-deploy` to `upstream/main`, then re-apply only the bits jim.tennis genuinely needs.

```bash
cd ~/Development/Tennis/competition-factory-server
git fetch upstream
git checkout jim-tennis-deploy
git reset --hard upstream/main
# now re-apply, as small focused commits:
```

What to re-apply:

| Item | Why kept |
|---|---|
| `Dockerfile` (deploy-branch version, with `pnpm@10.27.0` pin) | Upstream's compose comments out the server container — we still need our image built |
| `pnpm-workspace.yaml` jim.tennis-specific overrides (if any are still needed after pin) | Investigate whether pnpm-pinning is still required given upstream's recent pnpm-related cleanup |
| `scripts/parks-cup/*` + `src/scripts/parks-cup-*.ts` | Active import pipeline, not migrated to a different location upstream |
| `seed-admin.js` (root) + `.gitignore` entry | Production user seeding tool; not in upstream |
| `.env.production` values for jim.tennis | URL bindings, secrets |
| `docs/parks-cup-2026-import.md` | Local record of the import pipeline |

What to drop:

| Item | Why dropped |
|---|---|
| Cherry-picked `fix(users): normalize email to lowercase` (`b0b3b4d`) | Already in `upstream/main` as `cef5bc8` |
| `chore(deploy): pin pnpm@10.27.0 and move override into pnpm-workspace.yaml` (`4f6c9f5`) | Re-evaluate whether the pnpm pin is still needed against upstream's current `pnpm-workspace.yaml` |
| `fix: correct build output path in Dockerfile CMD` (`170de06`) | Will be subsumed into the new Dockerfile commit |
| `fix: allow saving tournaments without parentOrganisation` (`617ae05`) | Check if upstream's provider model changes have addressed this; if not, re-apply |

Push as a force-update to fork:
```bash
git push --force-with-lease origin jim-tennis-deploy
```

**Gate:** local build of the deploy-branch Dockerfile succeeds and the resulting image starts cleanly against a local Postgres.

---

## Phase 2 — Production preparation

### 2a. Server resize (if elected)

DigitalOcean panel → droplet → Resize → 1 GB → 2 GB. This is a short reboot. After:
- Verify all containers come back up: `docker compose ps`
- Check `free -h` shows ~2 GB total
- Health-check endpoints

### 2b. Off-server backup of LevelDB data

Belt-and-braces backup before any production change. The DigitalOcean snapshot is one layer; this is the other.

```bash
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." \
  > ~/backups/courthive-leveldb-$(date +%Y%m%d-%H%M%S).tar.gz
```

### 2c. Update production compose files

Edit `/opt/jim-dot-tennis/docker-compose.yml` (or the courthive-specific file, whichever is canonical) to add the `postgres` service, mirroring the upstream pattern. Add `depends_on: postgres: { condition: service_healthy }` to `courthive-server`. Add a `courthive-pg-data` named volume.

### 2d. Update `.env` on prod

Add to `/opt/jim-dot-tennis/.env`:
```
STORAGE_PROVIDER=postgres
PG_HOST=postgres
PG_PORT=5432
PG_USER=courthive
PG_PASSWORD=<generated strong password>
PG_DATABASE=courthive
```

Leave `DB_*` (LevelDB) vars in place during the migration so the script can still read from net-level-server. Remove them in Phase 4.

### 2e. Rsync the rebuilt deploy branch + build image

Per the corrected Section 4 in `PRODUCTION_QUICK_REFERENCE.md`:
```bash
cd ~/Development/Tennis/competition-factory-server
rsync -avz --exclude='node_modules' --exclude='dist' --exclude='.git' --exclude='build' \
  -e "ssh -i ~/.ssh/digital_ocean_ssh" \
  ./ root@144.126.228.64:/opt/competition-factory-server/
```

Don't restart courthive-server yet — just have the rsync'd source ready.

### 2f. DigitalOcean snapshot

Take a panel snapshot immediately before Phase 3. This is the cheapest rollback button.

---

## Phase 3 — Production cutover

**Announce maintenance.** Suggested message to users:
> CourtHive tournaments will be offline for ~30 min from HH:MM for a database migration. Jim.tennis league features (availability, fixtures, results) will remain available throughout.

**Cutover steps:**

```bash
# Step 1: Stop courthive-server (LevelDB net-level-server stays running for migration source)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose stop courthive-server"

# Step 2: Bring up Postgres (waits for healthy)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose up -d postgres"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker compose -f /opt/jim-dot-tennis/docker-compose.yml ps postgres"
# wait for (healthy)

# Step 3: Build the new courthive-server image (Postgres-aware)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose build courthive-server"

# Step 4: Start courthive-server briefly to let migration-runner apply
#         schemas 001-022 against the empty Postgres database
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose up -d courthive-server && sleep 15"
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker logs --tail 50 courthive-server | grep -i migration"

# Step 5: Run the data migration (LevelDB → Postgres)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "docker exec -it courthive-server node src/scripts/migrate-to-postgres.mjs --verbose"

# Step 6: Restart courthive-server so it picks up the migrated data cleanly
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose restart courthive-server"
```

**Smoke test checklist** (run all of these from a real browser, not curl):

- [ ] Sign in via TMX (`https://jim.tennis/tournaments`) with a known account
- [ ] Sign in with the email in **mixed case** — confirms the lowercase normalization survived the migration
- [ ] Open an existing tournament — draws render
- [ ] Open a match — scorecard loads
- [ ] Make a trivial mutation (assign a participant, save) — confirm it persists across a refresh
- [ ] Public viewer (`https://jim.tennis/public`) loads a published tournament
- [ ] Public viewer receives a live score update (open scoring on TMX in one tab, watch public in another)
- [ ] `https://jim.tennis/api/courthive/` returns `{"message":"Factory server"}`

**Gate:** if any smoke test fails and you can't see a quick fix → **rollback** (Phase 3b).

### Phase 3b — Rollback (only if smoke test fails)

```bash
# Flip env back to leveldb
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "sed -i 's/^STORAGE_PROVIDER=postgres/STORAGE_PROVIDER=leveldb/' /opt/jim-dot-tennis/.env"

# Restart with the old courthive-server image
# (might require checking out the previous image tag / rebuilding from the
#  pre-migration deploy-branch state — DigitalOcean snapshot is the
#  guaranteed reversal if image-tag fiddling fails)
ssh -i ~/.ssh/digital_ocean_ssh root@144.126.228.64 \
  "cd /opt/jim-dot-tennis && docker compose up -d courthive-server"
```

If image-level rollback is messy, fall back to the DigitalOcean snapshot — restores entire droplet to pre-migration state. ~5-10 min.

---

## Phase 4 — Cleanup (1-2 weeks after stable run)

Wait for at least one league night + a tournament weekend before declaring the migration stable.

- Remove net-level-server / LevelDB service from `docker-compose.yml`
- Drop the LevelDB Docker volume: `docker volume rm courthive-data`
- Remove `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS` from `.env`
- Update `PRODUCTION_QUICK_REFERENCE.md`:
  - Section 8 (Create CourtHive User): rewrite to use `psql` against the `users` table instead of net-level-client
  - Section 8b (Reset Password): rewrite similarly — much simpler, just a `bcrypt.hash()` and `UPDATE users SET password = $1 WHERE email = $2`
  - Section 4: remove the pnpm-pin note if upstream's pnpm setup no longer requires our pinning
- Off-server snapshot of the Postgres data dir for the new backup baseline

---

## Risks and mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Migration script bug — record shape mismatch | Medium | Phase 0 local POC catches this against a real data copy before prod |
| Memory pressure during cutover | Medium | Resize to 2 GB in Phase 2a; if not, lean on swap |
| Smoke test reveals a missing upstream feature we depend on | Low-medium | DigitalOcean snapshot rollback in Phase 3b |
| User session invalidation | Low (controllable) | Keep `JWT_SECRET` stable across cutover — existing tokens stay valid |
| Schema migrations fail on startup | Low | Run them in Phase 0 against a copy first; check logs in Step 4 of Phase 3 |
| Disk space (currently 79% full) | Low | 1.9 MB → low double-digit MB in Postgres + container overhead. Watch but unlikely to bite |

## Reference: where the data lives during each phase

| Phase | Source of truth | Notes |
|---|---|---|
| Pre-migration | LevelDB at `/app/data` in `courthive-server` container | net-level-server speaks the protocol |
| During migration (Phase 3, Steps 5-6) | Both LevelDB AND Postgres briefly | Script reads from LevelDB, writes to Postgres |
| Post-migration | Postgres in `courthive-postgres` container, volume `courthive-pg-data` | LevelDB volume retained but inert until Phase 4 |
| After Phase 4 | Postgres only | LevelDB volume + net-level-server service removed |

## Open work to do before scheduling Phase 3

- [ ] **Phase 0 local POC** — confirm migration script works against a real copy of prod data
- [ ] **Phase 1 deploy-branch rebuild** — clean reset + selective re-application
- [ ] **Phase 2a server resize decision** — confirm with cost/timing
- [ ] **Schedule cutover window** — pick a specific evening
- [ ] **Decide on JWT_SECRET handling** — keep stable (default) or rotate (forces re-login)
