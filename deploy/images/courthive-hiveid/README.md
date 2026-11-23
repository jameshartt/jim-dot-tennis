# courthive-hiveid image (jim-dot-tennis CourtHive bundle)

`ghcr.io/jameshartt/courthive-hiveid:<upstream version>-jdt.<n>`

HiveID is CourtHive's identity provider: login, sessions (access token plus a
rotating refresh token), user accounts, and the **only service that signs
tokens** (ES256, with a `kid` header). The tournament server (CFS,
competition-factory-server) and TMX are relying parties: they verify HiveID's
tokens with its public key and never sign their own. CFS asks HiveID to sign
tournament-scoped tokens through `POST /internal/mint`, guarded by a shared
service token.

Upstream: <https://github.com/CourtHive/courthive-hiveid> (MIT). There is **no
fork**. The Dockerfile here fetches the upstream repository at one exact
commit, checks that the checked-out commit is the one asked for, installs from
upstream's own `pnpm-lock.yaml` with `--frozen-lockfile`, builds, and copies the
build plus production dependencies into a small runtime image. No source file
is changed. This is not an official CourtHive build.

## Files

| File | Purpose |
| --- | --- |
| `Dockerfile` | Multi-stage build: `source` (git fetch + SHA/version checks + local-path guard), `build` (pnpm install, `nest build`, production install, licence list), runtime (`node:26-alpine`, user `node`). |
| `build.sh` | Builds the pinned commit locally under its final tag. Pushes nothing. |
| `third-party-licenses.mjs` | Copy of the bundle's shared licence script, run in `--node-modules` mode over the production dependency tree (145 packages; 4 ship no licence file and are listed with their declared licence: `@tokenizer/token`, `pg-types`, `pgpass`, `standardwebhooks`, all MIT). |
| `test/` | The throwaway compose test used for 0.5.0 (project `s023hiveid`; no host ports). Not needed to build. Secrets were generated into `test/secrets/` and deleted afterwards. |

## Build

```sh
./build.sh        # ghcr.io/jameshartt/courthive-hiveid:0.5.0-jdt.1
./build.sh 2      # same upstream commit, image revision 2 (0.5.0-jdt.2)
```

Build arguments (set by `build.sh`): `UPSTREAM_COMMIT` (full 40-character SHA),
`VERSION` (`<upstream version>-jdt.<n>`), optionally `UPSTREAM_REPO`. The build
fails if the SHA is not a full SHA, if the fetched commit differs, if `VERSION`
does not start with the upstream `package.json` version, or if `package.json`,
`pnpm-workspace.yaml` or `pnpm-lock.yaml` contain a `link:`/`file:`/`portal:`
specifier (the bundle builds only from the npm registry).

pnpm: the Node 26 images ship no corepack, so the build installs the exact pnpm
version upstream pins in `package.json` `packageManager` (`pnpm@12.16.1` at
0.5.0) with npm and checks the installed version. Dependency install scripts
are skipped (`--ignore-scripts`): nothing the service needs is native
(`bcryptjs` is pure JS) and husky's `prepare` hook needs a `.git`.

The image contains: `build/` (compiled service, email templates included),
production `node_modules/`, `package.json`, upstream's
`scripts/hash-plaintext-passwords.mjs` (only needed when upgrading a database
that still holds plaintext passwords; a fresh bundle install never does), and
`/licenses/` (`LICENSE-courthive-hiveid`, `NOTICE`, `THIRD-PARTY-LICENSES.txt`).
Labels: `org.opencontainers.image.{title,description,version,revision,source,licenses}`,
`jdt.upstream.{source,commit,version}`. `revision` is the upstream commit,
because nothing else was built.

## Moving to a newer upstream commit

1. Read upstream's `CHANGELOG.md`, `README.md` ("Self-hosting", "Routing") and
   `.env.example` between the old and new commit: new required settings, routes
   that move between HiveID and CFS, and **which CFS migrations the new commit
   expects**. HiveID creates no tables; it reads and writes CFS's schema. A
   HiveID newer than the bundled CFS can fail at runtime on a missing column
   (0.5.0 needs CFS migration `050-refresh-token-acting-provider.sql`; on a
   CFS without it every login returns 500).
2. In `build.sh` set `UPSTREAM_COMMIT` (full SHA) and `UPSTREAM_VERSION` (that
   commit's `package.json` version), and reset the revision to 1.
3. `./build.sh`. If the local-path guard fails, upstream has started pointing at
   sibling checkouts; decide then whether to add an explicit, named,
   fail-loud removal step (and record it in the NOTICE), rather than editing
   source silently.
4. Re-run the bundle's HiveID checks (login, refresh, `/auth/me`, CFS accepting
   the token, `/internal/mint` refusing without the service token).

## Settings the bundle must give it

HiveID reads environment variables only (it also loads a `.env` in `/app` if one
is mounted; the bundle should not rely on that).

| Variable | Value in the bundle |
| --- | --- |
| `NODE_ENV` | `production` (image default). In production HiveID refuses to boot without `HIVEID_SIGNER_TOKEN`; in development it boots and `/internal/mint` rejects every call. |
| `PORT` | `3140` (image default). |
| `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD`, `PG_DATABASE` | CFS's database. CFS's migrations must have run first. |
| `JWT_SIGN_ES256` | `true`. Without it HiveID signs **HS256** with `JWT_SECRET` (see below). |
| `JWT_PRIVATE_KEY` | ES256 (P-256) private key, PKCS#8 PEM; newlines may be written as `\n` in one line. HiveID only. |
| `JWT_PUBLIC_KEY`, `JWT_KID` | Matching public key (SPKI PEM) and a stable key id, e.g. `es256-<install date>`. Give the same two to CFS. Also served at `GET /.well-known/jwks.json`. |
| `JWT_ACCEPT_HS256` | `false`. Refuses HS256 tokens on every route. |
| `JWT_SECRET` | **Leave unset** on HiveID. When the ES256 key is missing or unreadable, HiveID silently falls back to HS256 signing with this secret; unset, that fallback fails (login 500) instead of issuing tokens nobody should accept. |
| `HIVEID_SIGNER_TOKEN` | Random secret; same value in CFS. |
| `APP_BASE_URL`, `TMX_URL`, `PUBLIC_URL`, `RESEND_API_KEY`, `EMAIL_FROM` | Leave unset: the bundle sends no email. Password reset, magic links and contact-email verification then fail or no-op (see below). A boot warning about `EMAIL_FROM` is expected. |
| `PERSONS_BASE_URL`, `PERSONS_SERVICE_TOKEN` | Leave unset (used only by `/auth/hiveid/signup`, which the proxy should refuse). |
| `JWT_PUBLIC_KEY_PREVIOUS`, `JWT_KID_PREVIOUS` | Only during a key rotation. |

HiveID does **not** use Redis.

Health: `GET /health` (liveness, reports the Node floor) and `GET /health/core-db`
(503 when the database is unreachable). The image's `HEALTHCHECK` uses
`/health/core-db`.

### Network and proxy

- Listen only on the internal network. Caddy sends `/api/courthive/auth/*` and
  `/api/courthive/account/*` here with `/api/courthive` stripped, **except**
  `/auth/tracker-token`, `/auth/scorer-token`, `/auth/provider-scoring-token`,
  `/auth/sso/*`, `/auth/hiveid/me/claimable/*` and `/auth/hiveid/me/claim`,
  which stay on CFS.
- `/internal/mint` must never be routed publicly; only CFS reaches it, at
  `HIVEID_BASE_URL` (e.g. `http://hiveid:3140`). `/health*` and
  `/.well-known/jwks.json` need not be public either.
- CFS needs `JWT_PUBLIC_KEY`, `JWT_KID`, `HIVEID_BASE_URL` and
  `HIVEID_SIGNER_TOKEN`; never the private key. **Also set
  `JWT_ACCEPT_HS256=false` on CFS.** CFS 4.19.1 refuses to boot without a
  non-empty `JWT_SECRET`, and by default it accepts HS256 tokens signed with
  that secret: in testing, a hand-made HS256 superadmin token signed with CFS's
  `JWT_SECRET` was accepted on `GET /admin/provisioners` (200). With
  `JWT_ACCEPT_HS256=false` the same token gets 401 and HiveID's ES256 tokens
  still work. Give CFS a random `JWT_SECRET` that nothing else uses.

### Behaviour without email or courthive-persons (tested, 0.5.0)

- `POST /auth/forgot-password`, `POST /auth/magic/request`: 200 `{ok:true}`,
  nothing sent (enumeration-safe; errors are swallowed and logged).
- `POST /account/contact-email/set`: 500 (`APP_BASE_URL is not set; cannot
  generate verification link`). Same family: `resend-verification`,
  `admin-resend`, `/auth/hiveid/resend-verification`, `/auth/hiveid/me/contact-email`.
- `POST /auth/admin-create-user` with a `contactEmail`: falls back to returning
  a generated password (`mode: "password-returned"`).
- `POST /auth/hiveid/signup`: 500 (`fetch failed`, persons defaults to
  `http://localhost:3100`).

### Fail-closed behaviour (tested, 0.5.0)

| Missing | Result |
| --- | --- |
| `HIVEID_SIGNER_TOKEN`, `NODE_ENV=production` | refuses to boot (exit 1, clear error) |
| `HIVEID_SIGNER_TOKEN`, `NODE_ENV=development` | boots; `/internal/mint` 401 even with a token |
| `JWT_PRIVATE_KEY` (or `JWT_SIGN_ES256` not `true`), no `JWT_SECRET` | boots, health 200; login 500; mint 503 `signer not configured` |
| `JWT_PRIVATE_KEY`, `JWT_SECRET` set | boots; login **issues HS256 tokens** (silent downgrade); mint 503 |
| all JWT keys | boots; JWKS empty; login 500; mint 503 |
| database unreachable / wrong password | boots; `/health` 200, `/health/core-db` 503 (container unhealthy); login 500 |

HiveID boots with no key and no database, so do not treat "container running"
as "ready": use the `/health/core-db` healthcheck, and check a login after
install.

### First administrator

HiveID has no bootstrap of its own. Create the first superadmin with CFS's
script (it writes a bcrypt hash), then log in through HiveID:

```sh
node src/scripts/admin-user.mjs create -e <email> -p <password>   # in the CFS container
```
