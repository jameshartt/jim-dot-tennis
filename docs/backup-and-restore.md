# Backups and restoring

The app backs itself up. This guide says what is in a backup, how to get a copy
off the server (the step that protects you if the server is lost), and how to
put a backup back. Practise a restore once, on a trial copy, before you need
it.

All server commands run in the install folder (see [Installing](install.md)):
log in with `ssh`, then `cd ~/jim-dot-tennis`.

## What is backed up

A backup is one file, named like
`jim-dot-tennis-backup-20261123T083111Z.tar.gz` (the date and time, in UTC). It
holds:

- the whole league database: the club, teams, players, fixtures, results,
  availability, accounts, password hashes and passkey details, settings and the
  app's own secrets (such as the key for phone notifications);
- the club logo you uploaded;
- **with tournaments on**: the whole tournament database (tournaments, draws,
  entries, results, tournament accounts);
- a list of the contents with a checksum for each file, so a damaged copy is
  noticed.

It does **not** hold:

- your `.env` settings file. Keep a copy of that yourself (below). With
  tournaments on it is essential: a tournament backup cannot be used on a new
  server without the secrets in `.env`.
- the site's security certificate (Caddy gets a new one by itself);
- the app's software (you download that again from the release).

### When backups are made

| Kind | When | Name starts |
|---|---|---|
| Daily | Every 24 hours, by itself; the first a few minutes after the app starts | `jim-dot-tennis-backup-` |
| By hand | When you run `jim-dot-tennis backup` | `jim-dot-tennis-backup-` |
| Download | When an administrator presses **Download backup** (made fresh, sent to the browser, not kept on the server) | `jim-dot-tennis-backup-` |
| Before an upgrade | Before a new version changes the database | `jim-dot-tennis-pre-migration-` |
| Before a tournament upgrade | Before a new version of the tournament software first starts | `jim-dot-tennis-pre-upgrade-` |

The app keeps the newest 14 daily and by-hand backups, plus the most recent
"before" backups (older ones are removed by later upgrades). They are in `/app/data/backups` inside the app's storage (the
Docker volume `jim-dot-tennis-data`), on the server's own disk.

You can change this in `.env`: `BACKUP_INTERVAL` (how often, for example `6h`,
or `off`), `BACKUP_KEEP` (how many) and `BACKUP_DIR` (the folder). After
changing `.env`, run `docker compose up -d` (a restart is not enough: see
[Troubleshooting](troubleshooting.md)).

### Keep backups private

A backup holds everyone's personal details, every password hash and passkey
record, and the app's secrets. Keep copies where only the people who run the
app can open them: not a shared drive, not email, not a group chat. Delete old
copies you no longer need.

## Check that backups are being made

In the app: the dashboard's **Backups** link (administrators only). It says
when the last backup was made, warns if that was too long ago, shows whether
tournament data is included, and lists the backups on the server.

On the server:

```
docker compose exec app ls -l /app/data/backups
```

You should see a list of `.tar.gz` files, the newest no more than a day old.

## Make a backup now

```
docker compose exec app ./jim-dot-tennis backup
```

You should see:

```
Backup made and checked:

    /app/data/backups/jim-dot-tennis-backup-20261123T083111Z.tar.gz

57959 bytes; database version 38; 0 uploaded file(s); app version X.Y.Z.
Does not include tournament data (this deployment has no tournament stack).
```

With tournaments on, the last line is instead
`Includes tournament data: a dump of the tournament database (...)`. If the
tournament data cannot be included, **no** backup is made and the command says
why: see "A backup fails with tournaments on" in
[Troubleshooting](troubleshooting.md).

Making a backup is safe while the app is in use.

## Download a backup in the browser

No terminal needed. Sign in as an administrator, open **Backups** from the
dashboard, and under **Download a backup** type your current password (or press
**Confirm with a passkey**), then press **Download backup**. Your browser saves
a fresh backup file. Downloads are recorded in the account audit log.

Store the file as described in "Keep backups private".

## Copy a backup off the server

The daily backups are on the same disk as the app: if the server is lost, they
are lost with it. At least once a week, and before any big change such as an
upgrade, put a copy somewhere else. The download button above is one way. From
the terminal:

1. On the server, make a fresh backup and note its file name:

   ```
   docker compose exec app ./jim-dot-tennis backup
   ```

2. On the server, copy it out of the app's storage into your home folder. Put
   the file name in place of `<file>`:

   ```
   docker compose cp app:/app/data/backups/<file> ~/<file>
   ```

   It ends `Copied ... to /root/<file>`.

3. **On your own computer**, in a new terminal, copy it down into the current
   folder:

   ```
   scp root@203.0.113.10:<file> .
   ```

4. Check the copy is whole. On your own computer:

   ```
   tar tzf <file>
   ```

   It should list `manifest.json` and `database/tennis.db` (and
   `tournaments/courthive.dump` with tournaments on), with no error.

5. Back on the server, delete the copy in your home folder:

   ```
   rm ~/<file>
   ```

Keep the last few copies and delete older ones. If you put them on a USB drive
or a cloud folder, use one that is encrypted and only yours.

### Keep a copy of your settings file too

Do this once after installing, and again after turning tournaments on. **On
your own computer:**

```
scp root@203.0.113.10:jim-dot-tennis/.env ./club-settings.env
```

Keep `club-settings.env` with your backups, just as privately: it holds the
tournament software's passwords and keys.

## Put a backup back (restore)

A restore replaces the app's current data with the backup's. Restoring only
works while the app is stopped. It checks the whole file first, keeps the
database it replaces (it never deletes it), and ends everyone's sign-in, so
everyone signs in again afterwards. Accounts, passwords and passkeys come back
as they were when the backup was made. Invite and recovery links that had not
been used are cancelled: make new ones.

### Restore, without tournaments

1. If the backup is not already on the server, copy it there. **On your own
   computer:**

   ```
   scp <file> root@203.0.113.10:
   ```

   Then on the server, put it in the app's backups folder and let the app read
   it:

   ```
   docker compose exec app mkdir -p /app/data/backups
   docker compose cp ~/<file> app:/app/data/backups/<file>
   docker compose exec -u root app chown appuser:appuser /app/data/backups/<file>
   ```

   (If it is one of the backups the app made, it is already there: skip this
   step. `docker compose exec app ls -l /app/data/backups` lists them.)

2. Stop the app:

   ```
   docker compose stop app
   ```

3. Restore, in a one-off copy of the app's container:

   ```
   docker compose run --rm --no-deps app ./jim-dot-tennis restore /app/data/backups/<file>
   ```

   You should see `Checking ... and restoring it into /app/data ...`, then
   `Restored the backup made ...`, and finally `Now start the app again.`

4. Start it again and check:

   ```
   docker compose up -d
   docker compose exec app ./jim-dot-tennis user list
   ```

   `user list` shows the accounts from the backup.

If step 3 says `Not restored: ...`, nothing was changed. Read the reason, put it
right and run step 3 again. The reasons you might see are in
[Troubleshooting](troubleshooting.md) ("A restore is refused").

### Restore, with tournaments on

The same steps, except that in step 2 the tournament server and identity
service are stopped as well (the tournament database stays running):

```
docker compose stop app courthive-server courthive-hiveid
docker compose run --rm --no-deps app ./jim-dot-tennis restore /app/data/backups/<file>
docker compose up -d
```

The restore output also says
`The tournament database was restored from the backup too.` The tournament
database is replaced in one step: if anything goes wrong it is left exactly as
it was.

- A backup **with** tournament data, restored where tournaments are off,
  brings back the league data only, and says the tournament data was not
  restored. To restore it later, turn tournaments on, stop the three as above,
  and run the restore with `restore --tournaments-only /app/data/backups/<file>`.
- A backup **without** tournament data (made with tournaments off, or a
  `pre-migration` backup) restores the league data and leaves the tournament
  data as it is.

### Tidy up after a restore

Each restore keeps what it replaced, as `tennis.db.replaced-<date>` (and
`courthive.dump.replaced-<date>` and `branding.replaced-<date>` where they
apply). Once you are sure the restore is right, see and remove them:

```
docker compose exec app sh -c 'ls -ld /app/data/*.replaced-*'
docker compose exec app sh -c 'rm -rf /app/data/*.replaced-*'
```

## If the server is lost: restore onto a new server

1. Follow [Installing](install.md) steps 1 to 5 on the new server (rent it,
   point the domain at it if its address changed, install Docker, download the
   release files of the **same version** the backup was made with, or a newer
   one).
2. Instead of step 6, put your saved settings file in place. **On your own
   computer:**

   ```
   scp ./club-settings.env root@203.0.113.10:jim-dot-tennis/.env
   ```

   Then on the server: `chmod 600 ~/jim-dot-tennis/.env`. This brings back
   your address and, with tournaments on, the tournament secrets that the
   tournament backup needs. Do not run `init-tournaments.sh` on the new server.
3. Start it (`docker compose pull` then `docker compose up -d`). Ignore the set-up
   code in the log.
4. Restore your newest copy as in "Restore, with tournaments on" (or without),
   including step 1 to copy the file up.
5. Open the site and sign in with your usual account.

If you have lost the settings file but have a backup with tournament data, you
can still restore: the league data comes back, but the app cannot sign in to
the tournament software. The fix is in [Troubleshooting](troubleshooting.md):
"The tournament sign-in service did not accept this app's service password".

## Practise a restore

Do this once, so you know it works before you need it. Use a trial copy on your
own computer ([Installing](install.md), "Trying it locally"), never your real
server:

1. Download a backup from your real site (Backups page, **Download backup**).
2. Start a trial copy. You do not need to finish its set-up.
3. Copy the backup in and restore it as above, from the trial folder:
   `docker compose cp <file> app:/app/data/backups/<file>`, the `chown`
   command, `docker compose stop app`, then the `restore` command and
   `docker compose up -d`.
4. Open `https://localhost` and sign in with your real username and password.
   (Passkeys from the real site do not work at `localhost`.)
5. Remove the trial with `docker compose down -v`: it holds real personal data.
