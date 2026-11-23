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
8. [Set-up cannot read the BHPLTA website](#8-set-up-cannot-read-the-bhplta-website)
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
19. ["service check-tournament-secrets didn't complete successfully"](#19-service-check-tournament-secrets-didnt-complete-successfully)
20. [Set-up made no season (between seasons)](#20-set-up-made-no-season-between-seasons)

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
error while interpolating services.caddy.environment.[]: required variable DOMAIN is missing a value: set DOMAIN in your .env file (the public hostname, e.g. tennis.example.org)
```

Nothing was started or changed.

**Fix:** you are not in the install folder (run `cd ~/jim-dot-tennis`), or
`.env` is missing or has no `DOMAIN=` value. Do step 6 of
[Installing](install.md).

**With the release files of version 1.0.0-rc.1 or older** (for example 0.2.1)
you may see this instead, even with tournaments off:

```
error while interpolating services.courthive-service-account.environment.PG_PASSWORD: required variable COURTHIVE_PG_PASSWORD is missing a value: COURTHIVE_PG_PASSWORD is missing from deploy/.env - run ./init-tournaments.sh
```

The name after `services.` and the end of the line can differ. It is about the
tournament secrets.

**Why:** those older files ask for the tournament secrets on every start, even
for a club that runs the league only, when the server has Docker Compose
version 2 (`docker compose version` says `v2...`). Newer release files do not.

**Fix:** run the secrets step once, in the install folder, then start again:

```
./init-tournaments.sh
docker compose up -d
```

This only adds the secrets to `.env`. It does not turn tournaments on, and the
secrets do no harm if you never use tournaments. Keep the lines in `.env`:
going back to these older files later needs them again.

With the newer files, missing tournament secrets show only when tournaments are
on, as problem 19.

If `./init-tournaments.sh` itself says
`init-tournaments: COURTHIVE_PG_PASSWORD is in .../.env but has no real value. Delete that line and run this again.`,
open `.env` in an editor (`nano .env`), delete that one line, save
(Ctrl+O, Enter, Ctrl+X) and run it again. It checks every line before it adds
anything, so nothing was added and there is nothing else to undo. It never
changes a line that is already there.

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

## 8. Set-up cannot read the BHPLTA website

**What you see** at "Choose your club", one of these:

```
Could not reach the BHPLTA fixtures page (...). This server could not connect to the BHPLTA website at all: check that it can connect to the internet and look up web addresses. Nothing was changed, so you can try again.
```

```
The BHPLTA website refused the request for the BHPLTA fixtures page (...). This server reached the website, so its internet connection is working. Wait a few minutes and try again. If it keeps happening, report it at https://github.com/jameshartt/jim-dot-tennis/issues. Nothing was changed, so you can try again.
```

The words in brackets give the detail. After "Could not reach" it is, for
example, `... no such host` or `... connection refused`. After "refused" it is
`HTTP status 403`, `HTTP status 429, too many requests`, or
`it reset or closed the connection (...)`.

**Why:** the club step reads the league from the BHPLTA website. "Could not
reach" means the server never got through to the website. "Refused" means the
website was reached but turned the request away.

**Fix for "Could not reach":** check the server can reach the website:

```
docker compose exec app sh -c 'wget -qO- https://www.bhplta.co.uk/ | head -c 60'
```

You should see the start of a web page (`<!DOCTYPE html>`). If you see an error
instead, the server has no internet access or no working DNS: check with your
hosting company. Then press **Try again**.

**Fix for "refused":** there is nothing wrong with your server. Press
**Try again** after a few minutes. If it keeps failing, report it on the
project's issue page, <https://github.com/jameshartt/jim-dot-tennis/issues>,
with the words in brackets (never include your set-up code).

If it says `Reading the BHPLTA fixtures page took too long`, the website is slow
or not answering: try again later.

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
`docker compose ps` shows the app `Restarting`. See why:

```
docker compose logs --no-log-prefix --tail 20 app
```

One of these:

- `Cannot start: the database was last used by a newer version of jim-dot-tennis than this one (...)`.
  An older version is being started on a database that a newer version has
  already upgraded. Nothing was changed. Either start the newer version again
  (set `JIM_DOT_TENNIS_VERSION` in your `.env` file back to it, then
  `docker compose up -d`), or, to go back to the older version, follow "If the
  database was upgraded" in [Upgrading](upgrade.md).
- `Cannot start: migration 0NN failed: ...` or
  `Cannot start: migration 0NN did not finish the last time the app started ...`.
  The upgrade itself failed part-way. The app never serves pages from a
  database in that state. Go back to the version you had before and restore the
  backup the app took before upgrading: follow "If the database was upgraded" in
  [Upgrading](upgrade.md). Then report the message on the project's issue page,
  <https://github.com/jameshartt/jim-dot-tennis/issues>.
- `Database migration failed, so the app cannot start: ...`. Something else
  stopped the upgrade before it changed anything. Report the whole message on
  the issue page.

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

(This step runs `jim-dot-tennis tournaments pre-upgrade` in a one-off copy of
the app's container; you never need to run it yourself.) Usually the disk is
full (`df -h /`) or the tournament database is not running.
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

## 19. "service check-tournament-secrets didn't complete successfully"

**What you see:** `docker compose up -d`, with tournaments on
(`COMPOSE_PROFILES=tournaments` in `.env`), ends with:

```
service "check-tournament-secrets" didn't complete successfully: exit 1
```

**Why:** before anything of the tournament part starts, a check makes sure its
secrets are in `.env`. One or more are missing, empty or a placeholder, so the
tournament part was not started or changed. The app and the league site carry
on as before. See which:

```
docker compose logs --no-log-prefix check-tournament-secrets
```

It names the missing secrets (never their values), for example
`... these tournament secrets are missing from .env, empty or a placeholder: COURTHIVE_PG_PASSWORD COURTHIVE_JWT_SECRET ...`.

**Fix:**

- On a new install, or when turning tournaments on for the first time: run
  `./init-tournaments.sh` in the install folder, then `docker compose up -d`.
- If tournaments were already in use and the lines have gone from `.env` (for
  example a new server, or `.env` made again): do **not** run
  `./init-tournaments.sh`. New secrets do not fit the tournament data you
  already have. Put back your saved copy of `.env` (see
  [Backups and restoring](backup-and-restore.md)), then `docker compose up -d`.
- If you meant to run the league only: put a `#` in front of the
  `COMPOSE_PROFILES=tournaments` line in `.env` and run `docker compose up -d`.

If `./init-tournaments.sh` says a line `has no real value`, see the end of
problem 2.

## 20. Set-up made no season (between seasons)

**What you see:** at the end of set-up, "You're all set" lists only the league
and the clubs, with the note
`BHPLTA has not published any upcoming fixtures yet, so no season was created. That is normal between seasons.`
Later, **Manage Seasons** says "No Seasons Found".

**Why:** set-up reads the season from the BHPLTA fixtures page, which lists
only upcoming fixtures. Between seasons it lists none, so there is no season to
bring in yet. Nothing is wrong and nothing needs undoing.

**Fix:** nothing now. Meanwhile you can add players and give them their links,
invite captains, set up Club Settings and turn on tournaments. When the new
season's fixtures are on the BHPLTA website, choose **Manage Seasons** on the
dashboard and press **Start a new season from BHPLTA**. If it again says no
season was created, the fixtures are not published yet: try again later.
