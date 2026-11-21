# Production Quick Reference - jim.tennis

**Server:** set in your own `scripts/.deploy.env` (`SERVER`, `SSH_KEY`, `REMOTE_PATH`; see `scripts/.deploy.env.example`). Commands below use these variables - `export` them (or `source scripts/.deploy.env`) first.
**Domain:** jim.tennis
**SSH:** `ssh -i $SSH_KEY $SERVER`
**Path:** `$REMOTE_PATH/`
**Reference deployment in git:** `deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/.env.example`. The server-side commands below use the file name the server runs today (`docker-compose.courthive.yml`); the server is reconciled with the reference in WI-141. `scripts/deploy-app.sh` does not sync compose or the Caddyfile.
**Go Version:** 1.26 (bumped from 1.25 on 2026-09-25; runtime image alpine:3.24)

---

## Quick Commands

### 1. Update Go Application (jim-dot-tennis)

**Preferred: use the script.** Wraps the rsync set, build, restart, and a health check.

```bash
cd ~/Development/Tennis/jim-dot-tennis
./scripts/deploy-app.sh             # full deploy: rsync + build + restart + verify
./scripts/deploy-app.sh --dry-run   # show what would change, no side effects
./scripts/deploy-app.sh --skip-build # rsync only (inspect on server before rebuilding)
```

**Or run the steps by hand (kept for reference / overrides):**

```bash
# Transfer and rebuild
cd ~/Development/Tennis/jim-dot-tennis
rsync -avz --delete -e "ssh -i $SSH_KEY" internal/ $SERVER:$REMOTE_PATH/internal/
rsync -avz --delete -e "ssh -i $SSH_KEY" cmd/ $SERVER:$REMOTE_PATH/cmd/
rsync -avz --delete -e "ssh -i $SSH_KEY" templates/ $SERVER:$REMOTE_PATH/templates/
rsync -avz --delete -e "ssh -i $SSH_KEY" static/ $SERVER:$REMOTE_PATH/static/
rsync -avz --delete -e "ssh -i $SSH_KEY" migrations/ $SERVER:$REMOTE_PATH/migrations/
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose build app && docker compose up -d app"
```

> **Do not forget `migrations/`!** Migrations are `COPY`'d into the Docker image at build time and run automatically on app startup. If new migrations exist in the source but are missing from the server, the build will bake in stale migration files and the app will fail with "no such column" errors at runtime. (The script always includes them.)

**Time:** ~2 minutes
**Downtime:** ~10 seconds

---

### 2. Update TMX Frontend

```bash
# Build and transfer
cd ~/Development/Tennis/TMX
git checkout jim-tennis-deploy   # MUST build from this branch — see warning below
nvm use 24
pnpm build                # Uses .env.production → https://jim.tennis/api/courthive
rsync -avz --delete -e "ssh -i $SSH_KEY" dist/ $SERVER:$REMOTE_PATH/TMX/dist/
```

> **⚠️ Build ONLY from the `jim-tennis-deploy` branch.** The production `.env.production`
> — with `SERVER=https://jim.tennis/api/courthive` and `BASE_URL=tournaments` — lives on
> that branch. `upstream/main` (and any feature branch cut from it) has an `.env.production`
> with **both values blank**, and a `pnpm build` from there silently produces a broken deploy:
> - **No `BASE_URL`** → relative `./assets/` paths. The app loads on `jim.tennis/tournaments/`
>   (trailing slash) but **white-screens on `jim.tennis/tournaments`** (no slash), because
>   `./assets/` resolves to `/assets/` at the root and hits the Go-app catch-all (`text/plain`
>   → blocked module). Confirm after building: `grep '<script' dist/index.html` must show
>   `src="/tournaments/assets/..."`, not `src="./assets/..."`.
> - **Blank `SERVER`** → login POSTs to `jim.tennis/auth/login` instead of
>   `.../api/courthive/auth/login` → **404 on login**. Confirm:
>   `grep -o 'https://jim.tennis/api/courthive' dist/assets/*.js` must return a match.
>
> A trailing-slash smoke test hides both bugs — always test the **no-trailing-slash** URL and
> an actual login. (History: cost several bad deploys on 2026-07-13.)

> **Remember to rebuild for local after deploying:** `pnpm build --mode development`
> and restart Caddy: `docker compose -f docker-compose.courthive.yml restart caddy`

**Time:** ~1 minute
**Downtime:** None

---

### 3. Update Public Frontend

```bash
# Build and transfer
cd ~/Development/Tennis/courthive-public
nvm use 20
pnpm build                # Uses .env.production → https://jim.tennis/api/courthive
rsync -avz --delete -e "ssh -i $SSH_KEY" dist/ $SERVER:$REMOTE_PATH/courthive-public/dist/
```

> **Remember to rebuild for local after deploying:** `pnpm build --mode development`
> and restart Caddy: `docker compose -f docker-compose.courthive.yml restart caddy`

**Time:** ~1 minute
**Downtime:** None

---

### 4. Update CourtHive Server

```bash
# Transfer and rebuild
cd ~/Development/Tennis/competition-factory-server
rsync -avz --delete --exclude='node_modules' --exclude='dist' --exclude='.git' --exclude='build' --exclude='backups' --exclude='.env' -e "ssh -i $SSH_KEY" ./ $SERVER:$REMOTE_PATH/competition-factory-server/
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose stop courthive-server && docker compose build courthive-server && docker compose up -d courthive-server"
```

> **Source path:** The compose files use `context: ../competition-factory-server` (relative to `$REMOTE_PATH/`), so the rsync target is `$REMOTE_PATH/competition-factory-server/` — sibling to `jim-dot-tennis/`, not inside it.

> **`--delete` is required (learned 2026-07-11):** without it, files removed from the repo linger on the server and get swept into the Docker build context — a stale `src/modules/email/email.service.ts` (removed upstream) broke `pnpm build` with phantom `mailgun.js` import errors. `--exclude='.env'` protects the server-local env file from deletion; `.env` lives only on the server and must never be clobbered.

> **Failed build ≠ outage:** if `docker compose build` fails, `up -d` restarts the previous image — the API comes back on the old version. Fix the build and re-run; nothing is lost.

> **Stop before build (1GB RAM droplet):** Stopping `courthive-server` first frees memory; without it the Nest build can OOM. Brings the ~15s downtime forward but avoids a stuck build.

> **pnpm via corepack:** The Dockerfile uses `corepack enable`, so pnpm follows the `packageManager` field in `package.json` (currently `pnpm@12.5.1`). If you bump pnpm, regenerate `pnpm-lock.yaml` locally first or the Docker `--frozen-lockfile` install will fail with `ERR_PNPM_LOCKFILE_CONFIG_MISMATCH` or refuse on `ignoredBuiltDependencies`.

> **⚠️ Calendar backfill required when upgrading past the calendar refactor (migrations 047/048, factory 7.x — learned 2026-09-25).** Migration `047-add-calendar-tournaments` creates an EMPTY `calendar_tournaments` table (the new one-row-per-tournament calendar); migration `048-drop-legacy-calendars` then REFUSES to drop the legacy `calendars` blob table unless the new table is populated — a deliberate data-safety guard (Button lost 5,844 calendar entries on 2026-09-21 when 047+048 ran in the same boot with no backfill). On a host that has both migrations applying in one deploy, the new server **crashes on startup** with `Migration 048-drop-legacy-calendars.sql failed`. Fix: run the backfill BETWEEN the two, then start the new image. The calendar is a projection, so it recomputes from tournament records:
> ```bash
> # From a one-off container off the NEW image (scripts/ is NOT in the prod image,
> # so bind-mount it from the rsync'd source; runtime image has node_modules + factory).
> ssh -i $SSH_KEY $SERVER '
>   umask 077
>   docker inspect courthive-server --format "{{range .Config.Env}}{{println .}}{{end}}" | grep -E "^PG_" > /tmp/pg.env
>   docker run --rm --network jim-dot-tennis_tennis-network --env-file /tmp/pg.env \
>     -v $REMOTE_PATH/competition-factory-server/scripts:/app/host-scripts:ro -w /app \
>     jim-dot-tennis-courthive-server:latest \
>     node host-scripts/backfill-calendar-tournaments.mjs --dry     # then --apply
>   rm -f /tmp/pg.env'
> ```
> Idempotent (upserts on `tournament_id`); `--dry` first. If the new image is already crash-looping, roll back fast by retagging the previous image: `docker tag <old-sha> jim-dot-tennis-courthive-server:latest && docker compose up -d --no-build --force-recreate courthive-server` (the old sha survives a rebuild as a dangling image — `docker images -f dangling=true`). Take a `pg_dumpall -U courthive` first.

**Time:** ~3 minutes (add ~1 min if a calendar backfill is needed)
**Downtime:** ~15 seconds
**Warning:** Interrupts active API users

---

### 5. View Logs

```bash
# All services
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose logs -f"

# Specific service (jim-dot-tennis, courthive-server, tennis-caddy, courthive-redis)
ssh -i $SSH_KEY $SERVER "docker logs -f jim-dot-tennis"
ssh -i $SSH_KEY $SERVER "docker logs --tail 50 courthive-server"
```

---

### 6. Check Status

```bash
# Container status
ssh -i $SSH_KEY $SERVER "docker compose ps"

# Resource usage
ssh -i $SSH_KEY $SERVER "docker stats --no-stream"

# Test endpoints
curl -I https://jim.tennis/
curl -I https://jim.tennis/admin/league
curl -I https://jim.tennis/tournaments
curl -I https://jim.tennis/public
curl https://jim.tennis/api/courthive/
```

---

### 7. Restart Services

```bash
# Restart specific service
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose restart app"
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose restart courthive-server"

# Restart all
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose restart"

# Full restart (down/up)
ssh -i $SSH_KEY $SERVER "cd $REMOTE_PATH && docker compose down && docker compose up -d"
```

---

### 8. Create CourtHive User

```bash
ssh -i $SSH_KEY $SERVER "docker exec -i courthive-server node" << 'EOFNODE'
const bcrypt = require('bcryptjs');
const netLevelClient = require('@gridspace/net-level-client');

async function createAdminUser() {
  const db = new netLevelClient();
  try {
    await db.open('localhost', 3838);
    await db.auth('<courthive-db-user>', '<courthive-db-password>');
    await db.use('user', { create: true });

    const email = 'newuser@example.com';  // CHANGE THIS
    const password = 'SecurePassword123!';  // CHANGE THIS

    const hashedPassword = await bcrypt.hash(password, 10);
    const user = {
      email,
      roles: ['superadmin', 'admin', 'developer', 'client', 'score'],
      permissions: ['devMode'],
      password: hashedPassword,
      firstName: 'New',
      lastName: 'User',
    };

    await db.put(email, user);
    console.log('User created: ' + email);
    await db.close();
  } catch (error) {
    console.error('Error:', error.message);
  }
}

createAdminUser();
EOFNODE
```

---

### 8b. Reset CourtHive User Password

For when someone has signed up via an invite link but can't log in. The most common cause was mobile keyboards capitalizing the first character of the email — fixed upstream in TMX `loginModal.ts` (`type: 'email'`) and `competition-factory-server` `users.service.ts` (lowercase normalization), but worth knowing about for any user who registered before those fixes shipped.

```bash
ssh -i $SSH_KEY $SERVER "docker exec -i courthive-server node" << 'EOFNODE'
const bcrypt = require('bcryptjs');
const netLevelClient = require('@gridspace/net-level-client');

async function resetPassword() {
  const db = new netLevelClient();
  try {
    await db.open('localhost', 3838);
    await db.auth('<courthive-db-user>', '<courthive-db-password>');
    await db.use('user', { create: true });

    const email = 'user@example.com';                // CHANGE THIS
    const newPassword = 'memorable-passphrase-XX';   // CHANGE THIS

    // db.get takes a key string directly; passing { key: email } returns nothing
    // and silently looks like "user not found". See src/scripts/admin-user.mjs.
    const existing = await db.get(email);
    if (!existing) {
      console.error('User not found: ' + email);
      await db.close();
      process.exit(1);
    }

    const user = typeof existing === 'string' ? JSON.parse(existing) : existing;
    user.password = await bcrypt.hash(newPassword, 10);

    await db.put(email, user);
    console.log('Password reset for: ' + email);
    console.log('Roles: ' + (user.roles || []).join(', '));
    await db.close();
  } catch (error) {
    console.error('Error:', error.message);
    process.exit(1);
  }
}

resetPassword();
EOFNODE
```

For a memorable-but-decent-entropy password, a 3–4 word passphrase like `marina-cobalt-piano-17` hits ~40–50 bits and reads easily over a call. Server-side enforcement is minimum 8 characters (TMX validator post-fix; see `TMX/src/components/validators/passwordValidator.ts`). Tell the user to change it from their profile page after first login.

To list users (useful to confirm the email casing in storage before resetting):

```bash
ssh -i $SSH_KEY $SERVER "docker exec -i courthive-server node" << 'EOFNODE'
const netLevelClient = require('@gridspace/net-level-client');
(async () => {
  const db = new netLevelClient();
  await db.open('localhost', 3838);
  await db.auth('<courthive-db-user>', '<courthive-db-password>');
  await db.use('user', { create: true });
  const all = await db.list({ all: true });
  for (const e of all || []) {
    console.log('  ' + (e?.key || e) + '  [' + ((e?.value?.roles) || []).join(',') + ']');
  }
  await db.close();
})();
EOFNODE
```

---

### 9. Backups

```bash
# List backups
ssh -i $SSH_KEY $SERVER "docker run --rm -v jim-dot-tennis-backups:/backups alpine ls -lh /backups"

# Manual backup
ssh -i $SSH_KEY $SERVER "docker run --rm -v jim-dot-tennis-data:/data -v jim-dot-tennis-backups:/backups alpine sh -c 'apk add sqlite && sqlite3 /data/tennis.db \".backup /backups/manual-\$(date +%Y%m%d-%H%M%S).db\"'"

# Download backup
ssh -i $SSH_KEY $SERVER "docker run --rm -v jim-dot-tennis-backups:/backups alpine cat /backups/tennis-YYYYMMDD-HHMMSS.db" > ~/backups/tennis-backup.db

# CourtHive data backup
ssh -i $SSH_KEY $SERVER "docker run --rm -v courthive-data:/data alpine tar czf - -C /data ." > ~/backups/courthive-$(date +%Y%m%d).tar.gz
```

---

### 10. Accounts and Setup from the Server Command Line

The app's own binary has commands for listing, creating, recovering and
disabling accounts, and for printing a new first-run setup code. Use them when
nobody can sign in as an administrator (section 12 walks through that). Run them inside the running container;
the app keeps running:

```bash
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user list"
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user reset <username>"
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis setup code"
```

Anyone who can run these commands already controls the server and the
database file. The commands add convenience, not privilege.

What they do and do not do:

- They use the same database as the server (`DB_PATH`, `/app/data/tennis.db` in
  the container) and wait briefly if the server is writing. They never create a
  database, apply migrations or run the startup checks. If the database is older
  than the program (an upgraded image that has not started yet) or a migration
  stopped part-way, they refuse and say so.
- `user create` and `user reset` never set or show a password. They print a
  single-use **link** (section 12): an invite link for a new account (works for
  48 hours), a recovery link for an existing one (works for 1 hour). The link is
  built from the public URL (`PUBLIC_URL`, or the Public URL on the Club Settings
  page); if none is set only the `/account/link/...` part is printed and you put
  the app's address in front of it yourself.
- `user reset` changes nothing by itself: the account keeps its password,
  passkeys and sessions until the link is used. `user disable` and
  `user sessions revoke` sign the account out everywhere at once.
- Disabling or demoting the last active administrator is refused.
- `setup code` only works while the app has no administrator. The new code
  replaces the one in the startup log and works once. The server's limit on
  wrong codes is not reset by it (restarting the app resets it).
- Each command records what it did (time, command, account, action) in the
  account audit log that administrators see at `/admin/league/audit-log`
  (section 13), and writes the same line to the terminal it ran in. No password,
  code or link is ever included.

The full `help` text:

```text
jim-dot-tennis - the club's tennis app

Run it with no command to start the web app. That is what the app's
container does, so you never need to start it by hand.

The commands below are for the person who looks after the club's server.
Run them on the server, inside the app's container, for example:

    docker exec -it jim-dot-tennis ./jim-dot-tennis user list

They work while the app is running; there is no need to stop it.

Accounts
  user list                        Show every account: its role, whether it can
                                   sign in, and when it last signed in.
  user create <username> --role <role>
                                   Make a new account. <role> is admin, captain
                                   or player. Prints an invite link once; the
                                   person who opens it sets up their own passkey
                                   or password. The link works once, for 48 hours.
  user reset <username>            For someone who is locked out (even the last
                                   administrator): prints a recovery link once.
                                   Whoever opens it sets a new passkey or
                                   password for the account and is signed in;
                                   the account is then signed out everywhere
                                   else. The link works once, for 1 hour. Nothing
                                   changes for the account until it is used.
  user disable <username>          Stop an account signing in, and sign it out
                                   everywhere. Refused for the last active
                                   administrator.
  user enable <username>           Let a disabled account sign in again.
  user role <username> <role>      Change an account's role. Refused if it would
                                   leave no active administrator.
  user sessions revoke <username>  Sign an account out on every device.

First-run setup
  setup code                       Print a new setup code, for creating the first
                                   administrator in the browser. Only works while
                                   the app has no administrator yet. It cancels
                                   any earlier code (including the one in the
                                   app's log) and works once.

  help                             Show this text.

Good to know
  - No command asks for, accepts, shows or sets a password. "user create" and
    "user reset" print a link instead. There is no email: pass the link on
    yourself, privately (in person, by phone or in a private message, not in a
    group chat). Anyone who opens it first gets the account.
  - A new link for an account cancels that account's earlier unused links.
  - Links start with the app's public address (PUBLIC_URL, or the Public URL
    on the Club Settings page). If none is set, only the part after the
    address is printed; put the address people use to reach the app in front
    of it, for example https://tennis.example.org/account/link/....
  - Anyone who can run these commands already controls the server and the
    app's database file. The commands make recovery easier; they do not give
    anyone powers they did not already have.
  - Each command records what it did: the time, the command and the account.
  - If the app was upgraded but has not started since, the commands refuse to
    run until it has (starting the app upgrades the database).
```

### 11. Passkeys

People sign in with a passkey (fingerprint, face or screen lock) or with a
username and password. The login page leads with "Sign in with a passkey"; the
password form stays underneath. Each person adds, names and removes their own
passkeys on the **Sign-in methods** page (`/account/signin-methods`, linked from
the dashboard header). A passkey that is someone's only way to sign in cannot be
removed. First-run setup offers "Create my login, then add a passkey".

**Passkeys are tied to the public URL.** A passkey only works at the web address
it was made for: the host of `PUBLIC_URL` (or the Public URL on the Club Settings
page) is the passkey "relying-party ID", and the whole origin (`https://host[:port]`)
is checked on every sign-in. The app reads the setting on every request, so a
change takes effect at once. Consequences:

- **Changing the domain (or moving between `www.` and the bare name) makes every
  existing passkey stop working.** Nothing is deleted, and changing it back makes
  them work again. Before changing it, make sure every administrator has a
  password; afterwards, anyone left without a way in gets a recovery link
  (section 12; `jim-dot-tennis user reset <username>` on the server if no
  administrator can sign in). Each person then adds a new passkey and removes
  the old one.
- The address must be the one people actually type. Opening the site by IP
  address or by another name gives a login page without the passkey button
  (browsers would refuse the passkey anyway). An IP address can never be a
  passkey address: use a host name.
- With no public URL configured (not possible in production, where the app
  refuses to start without one) passkeys are switched off and the login page
  shows only the password form. The request's `Host` header is never trusted
  for this.

**A passkey marked "Blocked".** Some authenticators count their uses. If a
passkey reports a count that went backwards, a copy of it may be in use: the
sign-in is refused, the passkey is blocked, a `SECURITY WARNING` line is logged
and administrators see a red banner on the dashboard. On the Sign-in methods
page an administrator can clear the warning after checking with the person, or
the person can remove the passkey and add a new one.

**Throttling.** Passkey attempts are limited per client address only (10 failed
sign-ins per 15 minutes, 30 started per minute), never per username, so nobody
can lock a named person out. Failed passkey attempts show in the dashboard's
login attempts as `(passkey)`.

**Nothing leaves the server.** No attestation is requested and no authenticator
metadata is fetched; `static/passkey.js` is the only script involved and is
served by the app.

**Local development.** WebAuthn needs https, with one exception: `http://localhost`.

```bash
# Simplest: no public URL, development mode, and open the app as localhost.
DEV_MODE=true            # already set by docker-compose.yml, make run-local, scripts/run.sh
# browse to http://localhost:8080  (not 127.0.0.1, not a LAN address or container name)

# Or pin it explicitly (also needed if you reach the app through a port mapping
# and want the address fixed):
DEV_MODE=true PUBLIC_URL=http://localhost:8080
```

With `DEV_MODE=true`, no public URL and a request addressed to `localhost`, the
app uses `http://localhost:<port>` as the passkey origin. Any other host name in
that situation gets no passkeys. `DEV_MODE` is refused in production, so this
never applies to a live site. The Playwright containers reach the app as
`http://app-e2e:8080`, which is not a passkey-capable address: the e2e suites
see the plain password form unless a test sets `PUBLIC_URL` and tells the
browser to treat that origin as secure.

### 12. Inviting and Recovering Accounts (Links, No Email)

**There is no email.** The app never sends anything to anyone. A new account is
handed over, and a locked-out account is recovered, with a **single-use link**
that an administrator (or the person who looks after the server) makes and
passes on themselves: in person, by phone or in a private message, not in a
group chat. Administrators never see, choose or reset anyone else's password.

**Inviting someone** (users page, `/admin/league/users`): enter a username and
role (and optionally the player record the account belongs to) and press
"Create user and invite link". The page shows the link **once**; copy it then.
Whoever opens it sees which account it is for, sets up a passkey (or a password
if they cannot use one) and is signed in. Invite links work for **48 hours**.
An account whose invite was never used shows "Invite not accepted" and a "New
invite link" button.

**Someone is locked out** (lost phone, forgotten password): press "Recovery
link" beside their name and pass the link on. Recovery links work for **1 hour**.
Making the link changes nothing for the account, so a mistaken click locks nobody
out. When the link is used the person sets a new passkey or password, is signed
in, and **every other session of the account is signed out**. On that page they
can tick "Remove my old ways of signing in" (do this when a device was lost or
stolen): the old passkeys and password are then deleted and only the new method
works. Unticked, existing passkeys are kept (a new password always replaces the
old one).

**Rules that always hold**

- A link works once and then never again. Opening the page does not use it up;
  finishing does.
- A new link for an account cancels that account's earlier unused links. Unused
  links are listed under "Links not used yet" on the users page, where "Cancel
  link" stops one working at once (do this if a link went to the wrong person).
- The link itself is shown only when it is made. Only a fingerprint (hash) of it
  is stored, so nobody can look it up later, not even in the database or a
  backup. If it is lost, make a new one.
- A deactivated account cannot use a link until it is activated again.
- Whoever opens the link first gets the account. Treat it like a password until
  it has been used.
- Making, cancelling and using a link are written to the account audit trail
  with who did it and for which account, never with the link.
- Guessing at link addresses is slowed down: after 10 wrong links in 15 minutes
  an address is refused for a while. A wrong, expired, used or cancelled link
  all show the same "This link can't be used" page.
- Links start with the public URL (`PUBLIC_URL`, or the Public URL on the Club
  Settings page). Passkeys are offered on the link page only when a public URL is
  set (section 11); otherwise the page asks for a password.
- Link addresses contain the secret, so they must not be written to access logs.
  The app never logs them, and the reference `deploy/Caddyfile` has no access
  log. If you add request logging to a proxy in front of the app, leave out (or
  redact) paths under `/account/link/`.

**Every administrator is locked out.** Nobody can press "Recovery link", so make
one on the server. This is the last resort and it needs access to the server,
which is the point: whoever controls the server already controls the database.

```bash
# Who is there, and which accounts are administrators?
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user list"

# Print a recovery link for an administrator (works once, for 1 hour)
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user reset <username>"

# If the account was deactivated, activate it first (or the link will not work)
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user enable <username>"

# If no administrator account is left at all, promote one or make a new one
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user role <username> admin"
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user create <username> --role admin"
```

Open the printed link in a browser, set a passkey or password, and you are
signed in as that administrator. If the app has no public URL set, the command
prints only the `/account/link/...` part: put the address you use to reach the
app in front of it.

**Upgrading to this version (migration 034).** Passwords become optional, so
`users.password_hash` may now be empty (NULL) for an account that signs in with
passkeys only or has not used its invite. Existing passwords, sessions and
passkeys are kept. One exception: an account still carrying a temporary password
that somebody else chose (made with an earlier `user create` / `user reset`, or
typed by an administrator, and never replaced) has that password removed; get the
person back in with `user reset <username>`. Rolling back (`cmd/migrate-down
-version 33`) gives accounts without a password an empty one that matches
nothing, and discards unused links.

### 13. Sessions, Sign-in Throttling, Audit Log and Cleanup

**Upgrading to this version (migration 035) signs everyone out once.** Sessions
used to be stored under the raw cookie token, so a copy of the database (a
backup, a droplet snapshot) held a working sign-in for everybody who was signed
in. Now the cookie carries a random token and the database stores only its
SHA-256 (`sessions.token_hash`); `sessions.id` is a plain number. The migration
deletes the old sessions instead of converting them, which also makes the raw
tokens inside every earlier backup useless. After the upgrade each person signs
in again (passkey or password); nothing else about their account changes.
Rolling back (`cmd/migrate-down -version 34`) signs everyone out again and drops
the audit log.

**Throttling of password sign-in.** Counted from `login_attempts` over 15
minutes:

- 5 failures for one username from one address block that username from that
  address only;
- 30 failures against any usernames from one address block password sign-in
  from that address (one password tried across many usernames).

Neither limit ever counts other addresses, so nobody can lock a named person out
of signing in from somewhere else, and neither applies to passkeys (section 11):
password failures against a name never block that person's passkey. While
blocked, further attempts are refused without being counted, so retrying does
not extend the block. An IPv6 client counts as its whole /64 network; the
dashboard's login attempts show that network for IPv6 clients.

**Client addresses.** Every throttle and recorded address comes from one
function (`clientIP` in `internal/auth/clientip.go`). It trusts the rightmost
`X-Forwarded-For` entry only when the direct peer is a private or loopback
address, i.e. the proxy. This is safe only while **the app's port is reachable
through Caddy alone**, as in `deploy/docker-compose.yml` (only Caddy publishes
ports). Never publish the app's port 8080 directly on a live server: Docker
would make every visitor appear to come from a private gateway address and
visitors could then choose their own address with that header.

**Account audit log** (`/admin/league/audit-log`, administrators only, linked
from the dashboard's System section). One row per account action, newest first,
50 per page: accounts created, invite and recovery links made, cancelled and
used, passwords changed, passkeys added, renamed, removed or blocked, roles
changed, accounts disabled and enabled, sessions revoked, commands run on the
server (`user ...`, `setup code`), and every administrator's sign-in (method and
address). Each row is written in the same database transaction as the change,
so there is no change without its row. Rows never contain a password, a token, a
hash or a session identifier. The page has no edit or delete, and the table
itself refuses UPDATE and DELETE (triggers). **Retention: kept for good**; the
rows are small. To prune it anyway, take a backup, then in `sqlite3` drop the
triggers `account_audit_log_no_update` and `account_audit_log_no_delete`, delete
what you must, and recreate the triggers from
`migrations/035_session_hashes_and_account_audit_log.up.sql`.

**Scheduled cleanup.** The app runs a cleanup a minute after it starts and then
every hour (`auth.Service.Cleanup`), and stops it when the app shuts down. It
deletes:

- sessions that are signed out, revoked, past their 7-day expiry or past the
  30-day absolute limit;
- login attempts older than 90 days;
- invite and recovery links that are used, cancelled or expired and were made
  more than 30 days ago.

It never deletes audit log rows. A run that deleted something writes one log
line with the counts.

**Logging.** Sign-in logs no usernames (attempted usernames are in
`login_attempts`, visible to administrators) and nothing per request; player
link tokens and the paths that contain them are never logged.

**Signing out** is a POST form (the Logout / Sign out buttons). `GET /logout`
answers 405 and signs nobody out, so a link or image on another page cannot
sign anyone out.

---

## Go Tooling (Sprint 004)

All commands run inside Docker (no local Go install required). Uses `golang:1.26-alpine`.

```bash
# Static analysis
make vet              # go vet ./...

# Formatting
make fmt              # Check formatting (list unformatted files)
make fmt-fix          # Fix formatting in-place

# Import ordering
make imports          # Check import ordering
make imports-fix      # Fix import ordering in-place

# Linting
make lint             # golangci-lint run ./...

# Dead code detection
make deadcode         # Find unreachable functions

# Module maintenance
make tidy             # go mod tidy

# Run all read-only checks at once
make check            # Runs: vet, fmt, lint, deadcode
```

---

## Admin Routes

### Core Admin (all under `/admin/league/`)

| Route | Purpose |
|---|---|
| `/admin/league/dashboard` | Dashboard - stat cards, quick actions (4 categories: League Management, Results & Standings, Season Tools, System) |
| `/admin/league/players` | Player management, filtering |
| `/admin/league/fixtures` | Fixture management, week overview |
| `/admin/league/teams` | Team management |
| `/admin/league/clubs` | Club management |
| `/admin/league/divisions/` | Division editing (Sprint 003) |
| `/admin/league/users` | User management - list, create, toggle active, password reset (Sprint 003) |
| `/admin/league/sessions` | Session management - view active, revoke (Sprint 003; sessions named by number, WI-131) |
| `/admin/league/audit-log` | Account audit log, read-only, administrators only (WI-131) |
| `/admin/league/points-table` | Points/standings table |
| `/admin/league/match-card-import` | BHPLTA match card import |
| `/admin/league/club-data-import` | Club data import |
| `/admin/league/seasons` | Season management, set active |
| `/admin/league/seasons/setup` | Season setup wizard |
| `/admin/league/selection-overview` | Selection overview |
| `/admin/league/wrapped` | Club Wrapped / season summary |
| `/admin/league/preferred-names` | Preferred name approvals |

### Public Routes

| Route | Purpose |
|---|---|
| `/club/wrapped` | Public Season Wrapped (access-cookie protected) |

---

## Service Architecture (Sprint 003)

`service.go` was split into 12 domain-specific files in `internal/admin/`:

| File | Domain |
|---|---|
| `service_dashboard.go` | Dashboard stats and overview |
| `service_players.go` | Player CRUD and queries |
| `service_fixtures.go` | Fixture management |
| `service_fixture_players.go` | Player-fixture assignments |
| `service_teams.go` | Team operations |
| `service_matchups.go` | Matchup logic |
| `service_seasons.go` | Season management |
| `service_divisions.go` | Division operations |
| `service_clubs.go` | Club management |
| `service_fantasy.go` | Fantasy league |
| `service_users_sessions.go` | User and session management |
| `service_selection.go` | Team selection logic |

---

## Troubleshooting

### Site not responding
```bash
ssh -i $SSH_KEY $SERVER "docker compose ps"
ssh -i $SSH_KEY $SERVER "docker logs --tail 50 tennis-caddy"
ssh -i $SSH_KEY $SERVER "docker compose restart caddy"
```

### API errors
```bash
ssh -i $SSH_KEY $SERVER "docker logs --tail 100 courthive-server"
ssh -i $SSH_KEY $SERVER "docker exec courthive-redis redis-cli ping"
ssh -i $SSH_KEY $SERVER "docker compose restart courthive-server"
```

### Admin login issues
```bash
ssh -i $SSH_KEY $SERVER "docker logs --tail 50 jim-dot-tennis | grep -i auth"
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ls -lh /app/data/tennis.db"
# Locked out of every administrator account: see section 12
ssh -i $SSH_KEY $SERVER "docker exec jim-dot-tennis ./jim-dot-tennis user list"
```

### TMX login issues
```bash
ssh -i $SSH_KEY $SERVER "docker exec courthive-server ps aux | grep net-level"
ssh -i $SSH_KEY $SERVER "docker exec courthive-server cat /app/data/.users"
```

---

## Credentials

No credentials are stored in this repository. Keep the real values in your own
password manager or an untracked operator runbook.

**jim-dot-tennis admin:** create the admin user for your instance; do not reuse a shared or default password.

**CourtHive/TMX admin:** created on the server by the operator (see `COURTHIVE_SETUP.md`). Store the email and password in your password manager.

**Environment secrets** (set in the server's `.env`, never committed):
- `WRAPPED_ACCESS_PASSWORD`: optional override for the Club Wrapped access code. Leave it unset and the instance generates its own code on first start (shown on the admin Club Settings page). A placeholder such as `change-me` stops the app starting in production.
- `COURTHIVE_JWT_SECRET`: must match `JWT_SECRET` of competition-factory-server (`.env` under `$REMOTE_PATH`)

---

## URLs

- **Main site:** https://jim.tennis/
- **League admin:** https://jim.tennis/admin/league
- **Tournament admin (TMX):** https://jim.tennis/tournaments
- **Public tournaments:** https://jim.tennis/public
- **CourtHive API:** https://jim.tennis/api/courthive/

---

## Build Times

- jim-dot-tennis: ~60-90 seconds
- courthive-server: ~90-120 seconds
- TMX: ~30-60 seconds
- courthive-public: ~20-40 seconds

---

## Server Constraints

The droplet is a **1 CPU / 1GB RAM** instance. Key operational constraints:

- **Swap:** 2GB swap file at `/swapfile` (added 2026-03-02, persisted in `/etc/fstab`). Without swap, Docker builds can OOM and lock up the server.
- **Docker builds must run one at a time.** Never run `docker compose build` (all services) — always build individually: `docker compose build app`, then `docker compose build courthive-server`. Parallel builds will peg the CPU at 100% and make SSH unresponsive.
- **Stop containers before building** to free memory: `docker compose down` before `docker compose build`.
- **If the server becomes unresponsive during a build**, the build is consuming all resources. Wait ~5-10 minutes or power cycle via the DigitalOcean dashboard.

## Resource Usage

- Total: ~210MB / 961MB (22%)
- jim-dot-tennis: ~20MB
- courthive-server: ~120MB
- Redis: ~8MB
- Caddy: ~15MB
