# Troubleshooting

The problems people most often meet, each with what you see and what to do.
Server commands run in the install folder (see [Installing](install.md)): log
in with `ssh`, then `cd ~/jim-dot-tennis`.

Two commands tell you most of what is going on:

```
docker compose ps
docker compose logs --tail 50 app
```

`ps` lists the containers and whether each is `Up`, `(healthy)`, `Restarting`
or `Exited`. `logs` shows the app's last 50 lines; put `caddy` (the web part)
or a tournament part such as `courthive-server` in place of `app` to see its
log instead.

Contents:

1. [I changed `.env` but nothing changed](#1-i-changed-env-but-nothing-changed)
2. [`docker compose` stops with "required variable ... is missing a value"](#2-docker-compose-stops-with-required-variable--is-missing-a-value)
3. [The app keeps restarting: "Configuration error" or "Startup self-check failed"](#3-the-app-keeps-restarting-configuration-error-or-startup-self-check-failed)
4. ["port is already allocated" or "address already in use"](#4-port-is-already-allocated-or-address-already-in-use)
5. [The site does not open, or the browser warns about the certificate](#5-the-site-does-not-open-or-the-browser-warns-about-the-certificate)
6. [The site has vanished after months of working (the domain lapsed)](#6-the-site-has-vanished-after-months-of-working-the-domain-lapsed)
7. [The set-up code is refused](#7-the-set-up-code-is-refused)
8. [Set-up says "Could not reach the BHPLTA fixtures page"](#8-set-up-says-could-not-reach-the-bhplta-fixtures-page)
9. [Passkeys stopped working, or the passkey button is missing](#9-passkeys-stopped-working-or-the-passkey-button-is-missing)
10. [The app keeps restarting after an upgrade](#10-the-app-keeps-restarting-after-an-upgrade)
11. [A restore is refused](#11-a-restore-is-refused)
12. [Tournament pages say "not switched on"](#12-tournament-pages-say-not-switched-on)
13. ["service courthive-pre-upgrade didn't complete successfully"](#13-service-courthive-pre-upgrade-didnt-complete-successfully)
14. [A backup fails with tournaments on](#14-a-backup-fails-with-tournaments-on)
15. ["The tournament sign-in service did not accept this app's service password"](#15-the-tournament-sign-in-service-did-not-accept-this-apps-service-password)
16. ["no matching manifest for linux/arm64"](#16-no-matching-manifest-for-linuxarm64)
17. ["permission denied while trying to connect to the Docker daemon socket"](#17-permission-denied-while-trying-to-connect-to-the-docker-daemon-socket)
18. [The server is slow or the tournament part keeps stopping](#18-the-server-is-slow-or-the-tournament-part-keeps-stopping)

## 1. I changed `.env` but nothing changed

**What you see:** you edited `.env` (for example `BACKUP_KEEP`, or added a
setting), ran `docker compose restart`, and the app behaves as before.

**Why:** a container reads its settings when it is created. `restart` stops
and starts the same container, with the old settings.

**Fix:** after changing `.env`, always run:

```
docker compose up -d
```

It recreates the containers whose settings changed (it says
`Container jim-dot-tennis Recreated`) and leaves the rest alone. To check a
setting took effect, for example `BACKUP_KEEP`:

```
docker compose exec app printenv BACKUP_KEEP
```

`docker compose restart app` is still fine when nothing in `.env` changed, for
example to print a new set-up code.

## 2. `docker compose` stops with "required variable ... is missing a value"

**What you see:**

```
error while interpolating services.caddy.environment.[]: required variable DOMAIN is missing a value: set DOMAIN in deploy/.env (the public hostname, e.g. tennis.example.org)
```

or, with tournaments on:

```
error while interpolating services.postgres.environment.POSTGRES_PASSWORD: required variable COURTHIVE_PG_PASSWORD is missing a value: COURTHIVE_PG_PASSWORD is missing from deploy/.env - run ./init-tournaments.sh
```

("deploy/.env" means the `.env` in your install folder.) Nothing was started or
changed.

**Fix:**

- `DOMAIN`: you are not in the install folder (run `cd ~/jim-dot-tennis`), or
  `.env` is missing or has no `DOMAIN=` value. Do step 6 of
  [Installing](install.md).
- A `COURTHIVE_...` name: tournaments are on (`COMPOSE_PROFILES=tournaments`
  in `.env`) but their secrets were never made. Run `./init-tournaments.sh`,
  then `docker compose up -d`.

If `./init-tournaments.sh` itself says
`init-tournaments: COURTHIVE_PG_PASSWORD is in .../.env but has no real value. Delete that line and run this again.`,
open `.env` in an editor (`nano .env`), delete that one line, save
(Ctrl+O, Enter, Ctrl+X) and run it again. It never changes a line that is
already there.

## 3. The app keeps restarting: "Configuration error" or "Startup self-check failed"

**What you see:** `docker compose up -d` ends with
`dependency failed to start: container jim-dot-tennis is unhealthy`, and
`docker compose ps` shows the app `Restarting`. The app's log says one of:

```
Configuration error: APP_ENV=production needs the instance's public address: set PUBLIC_URL=https://tennis.example.org in the app's environment (or enter it as the Public URL on the admin Club Settings page)
```

```
Configuration error: the public URL "http://tennis.example.org" must start with https:// in production; set PUBLIC_URL=https://tennis.example.org
```

```
Startup self-check failed: refusing to start in production: WRAPPED_ACCESS_PASSWORD set to a placeholder value from the documentation. Set a value of your own, or leave it unset so the app generates one for this instance
```

**Fix:** correct `.env`: `PUBLIC_URL=https://` followed by your address,
exactly as people type it; and leave `WRAPPED_ACCESS_PASSWORD=` empty unless you
chose a code of your own. Then `docker compose up -d`.

## 4. "port is already allocated" or "address already in use"

**What you see:** `docker compose up -d` stops with a line ending

```
Bind for 0.0.0.0:80 failed: port is already allocated
```

or

```
failed to bind host port 0.0.0.0:443/tcp: address already in use
```

**Why:** something else on the server already answers web requests on port 80
or 443, often a web server such as Apache or nginx that came with the server,
or another copy of this app.

**Fix:** on a server meant only for this app, stop and remove the other web
server (for Apache on Ubuntu: `sudo systemctl disable --now apache2`). On your
own computer, for a trial, use "If ports 80 and 443 are busy" in
[Installing](install.md).

## 5. The site does not open, or the browser warns about the certificate

**What you see:** the browser says "This site can't be reached", or "Your
connection is not private" (`NET::ERR_CERT_...`), on your real address.

Caddy gets the certificate from Let's Encrypt, which checks that your address
points at your server and that ports 80 and 443 are open. Check in this order:

1. **Is the app running?** `docker compose ps`: the app should be
   `(healthy)` and Caddy `Up`.
2. **Does the address point at the server?** On your own computer:
   `nslookup tennis.example.org` must show your server's address. If you added
   an `AAAA` record, it must be right too, or remove it.
3. **Are ports 80 and 443 open?** Check the firewall in your hosting
   company's control panel.
4. **What does Caddy say?**

   ```
   docker compose logs caddy | grep -i error
   ```

   Lines mentioning `challenge`, `NXDOMAIN`, `timeout` or `connection refused`
   point back to steps 2 and 3. A line mentioning `rateLimited` means too many
   failed tries: fix the cause and wait an hour.

After fixing DNS or the firewall, Caddy tries again by itself within minutes.
You can make it try at once with `docker compose restart caddy`.

On a trial with `DOMAIN=localhost` the certificate warning is expected: see
"Trying it locally" in [Installing](install.md).

## 6. The site has vanished after months of working (the domain lapsed)

**What you see:** suddenly nobody can open the site. The browser says "This
site can't be reached" (`DNS_PROBE_FINISHED_NXDOMAIN`), or shows a registrar's
"this domain has expired" or parking page. The server and app are fine:
`docker compose ps` shows everything `Up`.

**Why:** the domain's yearly renewal was missed. This has happened before, when
the renewal emails went to an inbox nobody read.

**Fix:**

1. Log in to the registrar's website and renew the domain. Most registrars
   keep an expired domain for you for a few weeks before anyone else can buy it.
2. Check the `A` record is still there (step 2 of [Installing](install.md)).
3. Wait for `nslookup tennis.example.org` to show your server again. Caddy
   renews the certificate by itself.

Then make it not happen again: turn on automatic renewal with a card that will
still be valid, make sure the registrar's emails go to an address someone
reads (not one person's old work address), and put the renewal date in the
club calendar with a reminder a month before.

## 7. The set-up code is refused

**What you see** on the set-up page:

- `That code doesn't match. Check you copied the latest code from the server log: each restart prints a new one.`
- or `The setup code has already been used, in another browser or an earlier visit.`

**Why:** each restart of the app, and each `setup code` command, makes a new
code and cancels the old one. `grep` may have shown an older code above the
newest. A code works once.

**Fix:** make a fresh code and use it straight away:

```
docker compose exec app ./jim-dot-tennis setup code
```

If you have tried several wrong codes, the page says
`Too many wrong codes have been tried. Wait 15 minutes and try again, or restart the app to get a new code.`
Wait, or restart the app (`docker compose restart app`), which resets the count
and prints a new code in the log:

```
docker compose logs --no-log-prefix app | grep "SETUP CODE" | tail -1
```

## 8. Set-up says "Could not reach the BHPLTA fixtures page"

**What you see** at "Choose your club":

```
Could not reach the BHPLTA fixtures page (...). Check that this server can connect to the internet. Nothing was changed, so you can try again.
```

The words in brackets give the detail, for example `no such host` or
`stream error: stream ID 1; PROTOCOL_ERROR; received from peer`.

**Why:** the club step reads the league from the BHPLTA website. Either the
server cannot reach it, or the website refused the request.

**Fix:**

1. Check the server can reach the website:

   ```
   docker compose exec app sh -c 'wget -qO- https://www.bhplta.co.uk/ | head -c 60'
   ```

   You should see the start of a web page (`<!DOCTYPE html>`). If you see an
   error instead, the server has no internet access or no working DNS: check
   with your hosting company.
2. If that works, the website is up but refused the app's request. Press
   **Try again** after a few minutes. If it keeps failing, report it on the
   project's issue page, <https://github.com/jameshartt/jim-dot-tennis/issues>,
   with the words in brackets (never include your set-up code).

Nothing is half-done: the club step either finishes or changes nothing. On a
trial copy you can carry on meanwhile with **Explore with demo data** on the
same page.

## 9. Passkeys stopped working, or the passkey button is missing

A passkey works only at the exact address it was made for: the host in
`PUBLIC_URL`.

**What you see:**

- After changing the domain (or moving between `www.` and the bare name),
  everyone's passkeys stop working: the browser offers none, or the sign-in
  fails.
- The login page has no "Sign in with a passkey" button, only the password
  form.

**Fix:**

- The button is missing when the site is opened at any address other than the
  one in `PUBLIC_URL`, for example by the server's numbers or another name. Use
  the real address.
- After a change of domain, nothing is deleted, and changing it back makes the
  old passkeys work again. To move for good: before changing, switch off
  "Require passkeys for administrators" in Club Settings and make sure every
  administrator has a password. After changing `PUBLIC_URL` (and `DOMAIN`) in
  `.env` and running `docker compose up -d`, each person signs in with their
  password, or with a recovery link
  ([Accounts and recovery](accounts-and-recovery.md)), adds a new passkey and
  removes the old one. If no administrator can sign in, use
  `docker compose exec app ./jim-dot-tennis user reset <username>`.
- Changing the domain also changes the identifiers in calendar files, so
  people who subscribed to the fixture calendar get duplicate events and
  should subscribe again.

Choose the address once and keep it.

## 10. The app keeps restarting after an upgrade

**What you see:** after an upgrade (or after going back to an older version),
`docker compose ps` shows the app `Restarting`, and its log says something like:

```
Database migration failed, so the app cannot start: failed to apply migrations: no migration found for version 38: read down for version 38 .: file does not exist
Nothing was served. Migrations read from: migrations built into this binary. See "Working with Migrations" in CLAUDE.md for a migration that stopped part-way.
```

**Why:** this exact message means an older version is being started on a
database a newer version already upgraded. Other `migration ... failed`
messages mean the upgrade itself failed part-way. The app never serves pages
from a database in that state.

**Fix:** go back to the version you had before and restore the backup the app
took before upgrading. Follow "If the database was upgraded" in
[Upgrading](upgrade.md). (The `CLAUDE.md` it mentions is a file for the app's
developers; you do not need it.)

## 11. A restore is refused

Every refusal starts `Not restored:` and ends `Nothing was changed.`

- `Not restored: the app is running: the app (or another restore) is using /app/data. Stop the app first, then run restore again in a one-off container ...`
  You ran `restore` with `docker compose exec`. Stop the app
  (`docker compose stop app`) and use `docker compose run --rm --no-deps app`
  as in [Backups and restoring](backup-and-restore.md).
- `Not restored: the tournament database is in use: 4 other connection(s) are open to the tournament database "courthive" on postgres:5432. The tournament server and the identity service must be stopped during a restore ...`
  Run `docker compose stop app courthive-server courthive-hiveid` and try again.
- A message about permission or reading the file: the copied file belongs to
  the wrong user. Run
  `docker compose exec -u root app chown appuser:appuser /app/data/backups/<file>`
  (with the app running), then stop it and try again.
- A message that the backup is from a newer version: restore it with that
  version or a later one (see [Upgrading](upgrade.md)).

## 12. Tournament pages say "not switched on"

- `/tournaments` shows only `Tournaments are not switched on for this site.`:
  the tournament part is not running. Check `.env` has the line
  `COMPOSE_PROFILES=tournaments` (with no `#` in front), then
  `docker compose up -d`.
- The tournament home page says `Tournaments are not switched on for this club at the moment.`:
  an administrator needs to tick **Tournaments** in Club Settings, Features,
  and save.

## 13. "service courthive-pre-upgrade didn't complete successfully"

**What you see:** `docker compose up -d` after an upgrade, with tournaments on,
ends with:

```
service "courthive-pre-upgrade" didn't complete successfully: exit 1
```

**Why:** before a new version of the tournament software first starts, a
backup of the tournament data is made. It could not be made, so the new
tournament server was not started and the tournament data was not touched. The
league site keeps working; tournament pages are unavailable until you act.

**Fix:** see why:

```
docker compose logs --no-log-prefix courthive-pre-upgrade
```

Usually the disk is full (`df -h /`) or the tournament database is not running.
Fix that and run `docker compose up -d` again. To get tournaments back on the
old version meanwhile, go back as in [Upgrading](upgrade.md) ("If the database
did not change"). Never start the tournament server with
`docker compose up --no-deps`: that skips this backup.

## 14. A backup fails with tournaments on

**What you see:** `jim-dot-tennis backup` says

```
Not done: the tournament database could not be backed up, so no backup was made (the backup must include it while the tournament stack is part of this deployment): the tournament database "courthive" on postgres:5432 could not be opened (is it running, and is its password right?): ...
```

and the Backups page says `The last scheduled backup failed: ...`.

**Why:** with tournaments on, a backup without the tournament data would look
complete but not be, so none is made. Usually the tournament database
(`courthive-postgres`) is stopped.

**Fix:** `docker compose up -d` (it starts what is stopped), check
`docker compose ps`, then make a backup by hand:
`docker compose exec app ./jim-dot-tennis backup`.

## 15. "The tournament sign-in service did not accept this app's service password"

**What you see** from tournament set-up (Club Settings, the Tournaments page, or
`tournaments provision`):

```
Stopped at: Connect this app to the tournament server.
The tournament sign-in service did not accept this app's service password. Check that COURTHIVE_SERVICE_PASSWORD is the password the tournament stack was set up with, restart the app, then try again.
```

**Why:** the tournament data was set up with different secrets from the ones
now in `.env`. This happens after restoring a backup onto a new server whose
`.env` was made fresh with `init-tournaments.sh`, or after editing the secrets.

**Fix:** put back the `.env` from the old server (your saved copy, see
[Backups and restoring](backup-and-restore.md)). If you only have the old
`COURTHIVE_SERVICE_PASSWORD=...` line, replace that one line in `.env` with
it. Then `docker compose up -d` (not `restart`: see problem 1) and run
`docker compose exec app ./jim-dot-tennis tournaments provision` again. Never
change the tournament secrets once tournaments are in use.

## 16. "no matching manifest for linux/arm64"

**What you see:** `docker compose pull` with tournaments on stops with

```
no matching manifest for linux/arm64 in the manifest list entries
```

**Why:** the tournament software is published for x86-64 (Intel or AMD)
servers only. The league app itself runs on ARM.

**Fix:** turn tournaments off on this server, or move to an x86-64 server
(restore your backup there: [Backups and restoring](backup-and-restore.md)).

## 17. "permission denied while trying to connect to the Docker daemon socket"

**What you see:** any `docker` command answers

```
permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock
```

**Why:** you are logged in as a user that may not use Docker.

**Fix:** log in as `root`, or put `sudo` in front of the command, or add your
user to the `docker` group (step 4 of [Installing](install.md)) and log in
again.

## 18. The server is slow or the tournament part keeps stopping

**What you see:** pages are slow, or `docker compose ps` shows tournament
containers restarting.

**Check** memory:

```
free -h
docker stats --no-stream
```

`free -h` shows the server's memory (`available` should not be near zero);
`docker stats` shows each container's use against its limit (the tournament
server 512 MB, identity service 256 MB, tournament database 384 MB, Redis
64 MB).

**Fix:** with tournaments on, use a server with 2 GB of memory (see
[Installing](install.md), "What you need"). Also check free disk space with
`df -h /`.
