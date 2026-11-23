# Upgrading to a newer version

An upgrade swaps the app for a newer version. Your data stays where it is. The
app makes a backup of its database before it changes anything, and the
tournament part (if you use it) does the same before its own upgrade.

The short version, once you have done it before: read the upgrade notes, keep
a copy of your current files, download the new files, set the version, then
`docker compose pull` and `docker compose up -d`.

All commands run on the server, in the install folder (see
[Installing](install.md)). Log in with `ssh` first.

## How you hear about new versions

The app never checks for updates by itself and never contacts the people who
make it. To hear about new versions, open
<https://github.com/jameshartt/jim-dot-tennis>, sign in to GitHub (a free
account), click **Watch**, then **Custom**, tick **Releases** and **Apply**.
GitHub then emails you when a version comes out. Or look at the releases page
from time to time: <https://github.com/jameshartt/jim-dot-tennis/releases>.

## Step 1: Find out which version you run

```
cd ~/jim-dot-tennis
docker compose exec app ./jim-dot-tennis version
```

You should see a line such as
`jim-dot-tennis 1.0.0 (commit 0cebf5d3fc3f, built 2026-11-23T07:55:18Z)`. The
number after `jim-dot-tennis` is your version.

Anyone can also see the version at `https://tennis.example.org/version`, and
administrators see it at the foot of the admin pages.

## Step 2: Read the upgrade notes

Each version has a section in the change log, with a part called
**Upgrade notes** that says what you need to know or do. Open it for the new
version (type the new version number in place of `X.Y.Z`):

`https://github.com/jameshartt/jim-dot-tennis/blob/vX.Y.Z/CHANGELOG.md`

Read the upgrade notes of **every** version between yours and the new one.
Most say "None: replace the image and restart." Look out for:

- a step you must do before or after;
- anything that says going back needs a backup (a database change);
- a change to the tournament software (a new "bundled set" table).

## Step 3: Back up and keep a copy of your current files

Make a backup now and copy it off the server, as in
[Backups and restoring](backup-and-restore.md) ("Make a backup now" and "Copy a
backup off the server"):

```
docker compose exec app ./jim-dot-tennis backup
```

It should start `Backup made and checked:` and give the file's name.

Then keep the files you run now, so you can go back to them:

```
mkdir -p previous
cp -p docker-compose.yml .env.example Caddyfile init-tournaments.sh .env previous/
ls -A previous
```

You should see the five file names. (`-p` keeps `.env` private.)

## Step 4: Download the new files and set the version

Type the new version number in place of `X.Y.Z`:

```
VERSION=X.Y.Z
for f in docker-compose.yml .env.example Caddyfile init-tournaments.sh; do curl -fsSLO "https://raw.githubusercontent.com/jameshartt/jim-dot-tennis/v$VERSION/deploy/$f"; done
chmod +x init-tournaments.sh
sed -i "s|^JIM_DOT_TENNIS_VERSION=.*|JIM_DOT_TENNIS_VERSION=$VERSION|" .env
grep JIM_DOT_TENNIS_VERSION .env
```

The last line should show `JIM_DOT_TENNIS_VERSION=` and the new number. If
`curl` says `The requested URL returned error: 404`, the number is wrong.

Your `.env` is not replaced: your settings and secrets stay as they are. To see
whether the new version added any settings, compare the old and new templates:

```
diff previous/.env.example .env.example
```

No output means nothing changed. Lines starting `>` are new in this version;
the upgrade notes say whether you need any of them.

## Step 5: Download and start the new version

```
docker compose pull
docker compose up -d
```

`pull` ends with lines saying `Pulled`. `up -d` replaces the app's container
(`Container jim-dot-tennis Recreated`, then `Healthy`). The site is unavailable
for a few seconds while it restarts.

With tournaments on, and if this version changes the tournament software, the
tournament server is started only after a backup of the tournament data has
been made and checked. That backup is
`jim-dot-tennis-pre-upgrade-<date and time>.tar.gz` in the backups folder.

## Step 6: Check it worked

```
docker compose ps
docker compose exec app ./jim-dot-tennis version
```

The app should be `Up ... (healthy)` and the version should be the new one.
Then look at what the app did as it started:

```
docker compose logs --since 10m app | grep -iE "backed up|migration|ready"
```

You should see one of:

- `No migrations to apply - database is up to date`: the database did not
  change.
- `Backed up the database before upgrading it: /app/data/backups/jim-dot-tennis-pre-migration-....tar.gz`
  then `Migrations applied successfully`: the database was upgraded, and the
  backup taken before it is named. Note its name.

and a last line `jim-dot-tennis X.Y.Z (...) ready: http://localhost:8080, ...`.
(That `localhost:8080` is the app's address inside its container, not the
address people use.)

With tournaments on, also check:

```
docker compose logs --no-log-prefix courthive-pre-upgrade
docker compose exec app ./jim-dot-tennis tournaments status
```

The first says either `Tournament server image unchanged` (nothing to do) or
that it made and checked a backup and `The tournament server may now start and
upgrade it.` The second should say `reachable`.

Finally open the site in a browser and sign in.

If the app does not come up, see "The app keeps restarting after an upgrade"
and "service courthive-pre-upgrade didn't complete successfully" in
[Troubleshooting](troubleshooting.md).

## Going back to the previous version (rolling back)

Which way depends on what step 6 showed.

### If the database did not change

The log said `No migrations to apply`. Put the previous files back and start
again:

```
cd ~/jim-dot-tennis
cp -p previous/docker-compose.yml previous/.env.example previous/Caddyfile previous/init-tournaments.sh previous/.env .
docker compose up -d
docker compose exec app ./jim-dot-tennis version
```

The version should be the old one again.

### If the database was upgraded

The log said `Backed up the database before upgrading it`. An older version
cannot use a database a newer one has upgraded: just putting the old version
back makes the app stop at start with `Database migration failed, so the app
cannot start: failed to apply migrations: no migration found for version ...`.
Instead, restore the backup taken just before the upgrade, using the old
version:

1. Put the previous files back (this sets the old version):

   ```
   cd ~/jim-dot-tennis
   cp -p previous/docker-compose.yml previous/.env.example previous/Caddyfile previous/init-tournaments.sh previous/.env .
   ```

2. Find the backup taken before the upgrade. Its name starts
   `jim-dot-tennis-pre-migration-`:

   ```
   docker compose run --rm --no-deps app ls -t /app/data/backups
   ```

   The newest files come first.

3. Stop the app and restore that backup with the old version. Put the file's
   name in place of `<file>`:

   ```
   docker compose stop app
   docker compose run --rm --no-deps app ./jim-dot-tennis restore /app/data/backups/<file>
   ```

   It should end `Now start the app again.` (It says the backup was made by
   the new version: that is right. The new version made it, of the database
   as it was before the upgrade.)

4. Start everything and check:

   ```
   docker compose up -d
   docker compose exec app ./jim-dot-tennis version
   ```

Everyone has to sign in again afterwards. Anything entered in the app since
the upgrade is lost, so roll back soon or not at all.

### If the tournament software was upgraded too

Use the same steps, but in step 2 pick the newest
`jim-dot-tennis-pre-upgrade-` file (it holds the league and the tournament
data), and in step 3 stop the tournament server and identity service as well:

```
docker compose stop app courthive-server courthive-hiveid
docker compose run --rm --no-deps app ./jim-dot-tennis restore /app/data/backups/<file>
docker compose up -d
```

It should say `The tournament database was restored from the backup too.`

More about restores, and what to do if one is refused: [Backups and
restoring](backup-and-restore.md).

## Good to know

- **Never** start the tournament server on its own with
  `docker compose up --no-deps`: that skips the backup taken before a
  tournament upgrade.
- Do not change the four `COURTHIVE_..._VERSION` settings in `.env`. Each
  version of the app is tested with one set of tournament software and brings
  it with it; leave them empty.
- The old version's files stay in `previous/` until your next upgrade
  replaces them. `previous/.env` holds your secrets: keep it as private as
  `.env`.
- Old versions of the app stay on the server's disk after an upgrade. Check
  free space now and then with `df -h /` (the `Use%` of `/` should stay well
  under 90%).
