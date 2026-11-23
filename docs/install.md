# Installing the app for your club

This guide takes you from nothing to a working copy of the app for your club,
on a small server you rent, at a web address you choose. You do not need to
have used Docker before. Every command is written out for you to copy and
paste.

Plan on about an hour, most of it waiting for things to download.

If something does not look the way this guide says it should, see
[Troubleshooting](troubleshooting.md).

Other guides you will want later:

- [Upgrading](upgrade.md) to a newer version
- [Backups and restoring](backup-and-restore.md)
- [Accounts and recovery](accounts-and-recovery.md): getting people back in from the server
- [Privacy and your club's data](privacy-and-data.md)
- [The administrator's guide](guide-admin.md), for running the app once it is installed

## A few words you will meet

- **Server**: a computer that is always on and connected to the internet.
  You rent one from a hosting company and control it from your own computer.
- **Terminal**: a window where you type commands. On Windows it is called
  PowerShell or Terminal; on a Mac it is Terminal (in Applications, Utilities).
- **SSH**: how your terminal logs in to the server.
- **Docker**: free software that runs the app in sealed boxes called
  **containers**, so you do not have to install the app's parts one by one.
- **Docker Compose**: the part of Docker that starts several containers
  together from one file, `docker-compose.yml`.
- **`.env` file**: a small text file of settings for your installation, such
  as your web address.
- **Domain**: a web address such as `tennis.example.org`.

In this guide `tennis.example.org` stands for your club's address and
`203.0.113.10` stands for your server's address. Put your own in their place.

## What you need

### A server

Rent a "virtual private server" (sometimes called a VPS, droplet or cloud
server) from any hosting company. These sizes come from measurements of the
app and its tournament software running:

| | Memory (RAM) | Disk | Processor |
|---|---|---|---|
| League only | 1 GB | 20 GB | 1 virtual CPU |
| League and tournaments | 2 GB (or 1 GB with 2 GB of swap) | 20 GB | 1 virtual CPU, **x86-64 (Intel or AMD) only** |

- Choose **Ubuntu 24.04 LTS** as the operating system. (Ubuntu 22.04 LTS and
  26.04 LTS also work with the Docker instructions below.)
- If you might ever want tournaments, choose an **x86-64** (also called
  amd64, Intel or AMD) server. The league app also runs on ARM servers, but the
  tournament software is published for x86-64 only.
- Choose a data centre in the UK or nearby.

Rough cost at the time of writing: about £4 to £6 a month for a 1 GB server,
and about £8 to £12 a month for a 2 GB server. Prices vary between companies.

The hosting company will give you the server's address (four numbers with dots,
like `203.0.113.10`) and a way to log in: either a password, or an "SSH key"
you add when you create the server. Follow the company's own instructions for
that part.

**Firewall.** If the hosting company has a firewall in its control panel, allow
incoming connections on ports **22** (SSH, for you), **80** and **443** (the
web). Nothing else needs to be open.

### A domain name

You need a web address for the app, for example `tennis.yourclub.org.uk`. Two
common ways:

- Use a part of the club's existing domain (a "subdomain"), such as
  `tennis.yourclub.org.uk`. Ask whoever looks after the club's website to add
  the record in step 2.
- Buy a new domain from a domain registrar (roughly £10 to £50 a year).

**Domains expire.** A domain is rented for a year or more at a time. If the
renewal is missed, the site disappears. Turn on automatic renewal, make sure
the renewal emails go to an address someone reads, and put the renewal date in
the club calendar. See "The site has vanished after months of working" in
[Troubleshooting](troubleshooting.md).

**Choose the address once.** Passkeys (sign-in with a fingerprint, face or
screen lock) are tied to the exact address. Changing it later means everyone
has to set up their passkeys again.

### Your own computer

Any Windows, Mac or Linux computer with a terminal. Windows 10 and 11 and
every Mac already include the `ssh` and `scp` commands this guide uses.

## Step 1: Rent the server

Create the server at your hosting company with the size and operating system
above. Write down its address.

## Step 2: Point your domain at the server

In the control panel of whoever manages your domain (the registrar, or the
company that hosts the club's website), add a DNS record:

| Type | Name | Value |
|---|---|---|
| `A` | `tennis` (the part before your domain; use `@` for the whole domain) | your server's address, for example `203.0.113.10` |

Leave any "TTL" setting at its default.

Only add an `AAAA` record (an IPv6 address) if you know the server's IPv6
address. A wrong `AAAA` record stops the site getting its security certificate.

Changes can take from a few minutes to a few hours to spread. Check on your own
computer:

```
nslookup tennis.example.org
```

You should see your server's address in the answer, for example:

```
Name:	tennis.example.org
Address: 203.0.113.10
```

If you see a different address, or "can't find" / "NXDOMAIN", wait a while and
try again. Do not carry on to step 7 until it shows the right address: the
site cannot get its certificate before then.

## Step 3: Log in to the server

In the terminal on your own computer:

```
ssh root@203.0.113.10
```

The first time, it asks `Are you sure you want to continue connecting
(yes/no/[fingerprint])?`. Type `yes` and press Enter. Then type the password, or
it uses your SSH key. You should end up at a prompt like
`root@your-server:~#`.

Everything from here on is typed into this window, on the server, unless the
guide says "on your own computer".

If your hosting company gave you a user other than `root`, log in as that user.
The commands below that start with `sudo` work for either.

## Step 4: Install Docker

These are Docker's own instructions for Ubuntu
(<https://docs.docker.com/engine/install/ubuntu/>). Copy and paste each block
in turn.

Add Docker's software source:

```
sudo apt update
sudo apt install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

sudo tee /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

sudo apt update
```

Install Docker:

```
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

Check it works:

```
sudo docker run hello-world
```

You should see `Hello from Docker!` among the lines it prints.

**If you are not logged in as `root`**, let your user run Docker without
`sudo`, then log out and back in:

```
sudo usermod -aG docker $USER
exit
```

Log in again with `ssh` as in step 3. (Being in the `docker` group gives the
same power as `root` on this server, so only add people who look after it.)

Check the Compose part:

```
docker compose version
```

You should see `Docker Compose version v2...` or higher.

## Step 5: Download the release files

Each version of the app is a "release". The list is at
<https://github.com/jameshartt/jim-dot-tennis/releases>. Note the version
number of the latest one, for example `1.0.0` (the release is labelled
`v1.0.0`; use the number without the `v`).

Make a folder for the app and download four files into it. In the second line,
type the latest version number in place of `X.Y.Z`:

```
mkdir -p ~/jim-dot-tennis && cd ~/jim-dot-tennis
VERSION=X.Y.Z
for f in docker-compose.yml .env.example Caddyfile init-tournaments.sh; do curl -fsSLO "https://raw.githubusercontent.com/jameshartt/jim-dot-tennis/v$VERSION/deploy/$f"; done
chmod +x init-tournaments.sh
ls -A
```

You should see the four files:

```
.env.example  Caddyfile  docker-compose.yml  init-tournaments.sh
```

If you see `curl: (22) The requested URL returned error: 404`, the version
number is wrong: check it on the releases page (no `v`, and no spaces).

What they are:

- `docker-compose.yml` says which containers to run.
- `.env.example` is the settings template.
- `Caddyfile` configures Caddy, the part that answers web requests and gets
  the site's security certificate.
- `init-tournaments.sh` is only used if you turn tournaments on.

This folder, `~/jim-dot-tennis`, is your **install folder**. Run every
`docker compose` command in it (`cd ~/jim-dot-tennis` first if you have logged
in again). Messages from the app call it "the folder holding
docker-compose.yml".

## Step 6: Make your settings file

Copy the template, keep it private, and put your address in it. In the third
line, type your own address in place of `tennis.example.org`:

```
cp .env.example .env
chmod 600 .env
CLUB_DOMAIN=tennis.example.org
sed -i "s|^DOMAIN=.*|DOMAIN=$CLUB_DOMAIN|; s|^PUBLIC_URL=.*|PUBLIC_URL=https://$CLUB_DOMAIN|" .env
grep -E '^(DOMAIN|PUBLIC_URL|JIM_DOT_TENNIS_VERSION)=' .env
```

You should see three lines, with your address and the version you downloaded:

```
DOMAIN=tennis.example.org
JIM_DOT_TENNIS_VERSION=X.Y.Z
PUBLIC_URL=https://tennis.example.org
```

That is all you need to set. There are no passwords to choose here: the app
has no built-in accounts or passwords, and makes its own secrets when it first
starts. The other settings in `.env` are explained in the file itself and are
all optional.

## Step 7: Start the app

```
docker compose pull
docker compose up -d
```

`pull` downloads the app and Caddy (a few minutes the first time; each line
ends `Pulled`). `up -d` starts them and finishes with lines like:

```
 Container jim-dot-tennis Healthy
 Container jim-dot-tennis-caddy Started
```

Check both are running:

```
docker compose ps
```

You should see `jim-dot-tennis` with `Up ... (healthy)` and
`jim-dot-tennis-caddy` with `Up`.

Caddy now asks Let's Encrypt (a free certificate service) for the site's
security certificate. That takes up to a minute.

## Step 8: Read the one-time set-up code

The app has no administrator yet. To prove you are the person who installed
it, it printed a one-time code in its log:

```
docker compose logs --no-log-prefix app | grep "SETUP CODE" | tail -1
```

You should see one line like this (your code will be different):

```
  SETUP CODE: 7KQ4-M2XD-9HTA
```

Keep it to yourself until you have used it. It works once.

- If you see nothing, wait ten seconds and run the command again.
- If you lose the code, or it says it has already been used, make a new one.
  This cancels the old code:

  ```
  docker compose exec app ./jim-dot-tennis setup code
  ```

  It prints `Setup code (shown only this once):` and the new code. (Restarting
  the app with `docker compose restart app` also prints a new one in the log.)

## Step 9: Finish set-up in your browser

Open `https://tennis.example.org` (your address) in a browser. You should see
a padlock in the address bar and a page headed "Welcome to ...", saying "This
app is running but has not been set up yet."

If the browser says the site cannot be reached, or warns that the connection
is not private, see "The site does not open, or the browser warns about the
certificate" in [Troubleshooting](troubleshooting.md).

1. **Setup code.** Type or paste the code and press **Continue**. Capitals,
   spaces and dashes do not matter.
2. **Create your login.** Choose a username (for example `sam.jones`) and a
   password of at least 12 characters, and press **Create my login**. If your
   browser offers **Create my login, then add a passkey**, you can use that
   instead to sign in with your fingerprint, face or screen lock in future.
3. **Choose your club.** Pick your club from the list of BHPLTA clubs and press
   **Use this club**. The page says "Fetching your league from the BHPLTA
   website" and updates by itself. This usually takes under a minute.
4. **You're all set.** The page lists what was set up: the league, the season,
   divisions, teams (and how many are yours), weeks and fixtures. Press
   **Go to the dashboard**.

The set-up pages are now switched off for good. You are the first
administrator.

If the club step cannot read the BHPLTA website, see "Set-up cannot read the
BHPLTA website" in [Troubleshooting](troubleshooting.md). Its message says
whether the server could not connect at all or the website refused.

## Step 10: The first things to do

- **Add a passkey** if you did not at set-up: the "Sign-in methods" link at the
  top of the dashboard.
- **Look at Club Settings** (on the dashboard): your club's name, logo and
  colours, and your club's BHPLTA club code for importing match cards.
- **Invite your captains** from the users page. See the
  [administrator's guide](guide-admin.md).
- **Copy your settings file and a backup off the server.** See
  [Backups and restoring](backup-and-restore.md). The app backs itself up every
  day, but those copies are on the same server.
- **Note your domain's renewal date.**

## Optional: turning on tournaments

The app can run your club's tournaments (club championships, for example) with
the bundled tournament software, CourtHive. It is off unless you turn it on.
It needs a 2 GB, x86-64 server (see "What you need").

You can turn it on straight after installing or at any time later. On the
server, in the install folder:

```
cd ~/jim-dot-tennis
./init-tournaments.sh
```

This makes the tournament software's passwords and keys, on your server, and
adds them to `.env`. It never shows them. You should see seven lines starting
`init-tournaments: added`, then
`init-tournaments: added 7 variable(s) to /root/jim-dot-tennis/.env (mode 600).`
and a short "Next" list. Run it once; running it again changes nothing.

Turn the tournament part on and start it:

```
echo COMPOSE_PROFILES=tournaments >> .env
docker compose pull
docker compose up -d
```

The download is about 1.5 GB in all, so `pull` takes a while. `up -d` then
starts the tournament containers in order. Some of them do one job and stop;
that is expected. Check after a minute:

```
docker compose ps
```

You should see these running, the last two marked `(healthy)` like the app:
`jim-dot-tennis`, `jim-dot-tennis-caddy`, `courthive-redis`,
`courthive-postgres`, `courthive-server`, `courthive-hiveid`.

Now switch tournaments on in the app:

1. Sign in, open **Club Settings** from the dashboard, and find **Features**.
2. Tick **Tournaments**. Underneath, check **Club's tournament code** (2 to 12
   letters or digits, made from your club's name). It cannot be changed once
   set up.
3. Press **Save** at the bottom of the page.

You should see "Tournaments are ready. The club is set up on the tournament
software as ..." with your code. You can also check from the server:

```
docker compose exec app ./jim-dot-tennis tournaments status
```

It should say `Bundled tournament software: switched on.` and
`Club set up: yes, with the tournament code ...`.

Who can run tournaments is set per person on the users page, or from the
server ([Accounts and recovery](accounts-and-recovery.md)). How to run a
tournament is in the [tournaments guide](guide-tournaments.md).

From now on every backup also includes the tournament data.

### Turning tournaments off again

Untick **Tournaments** in Club Settings and save. Then, on the server, put a `#`
in front of the `COMPOSE_PROFILES=tournaments` line in `.env` and stop the
tournament containers:

```
cd ~/jim-dot-tennis
sed -i 's/^COMPOSE_PROFILES=tournaments$/#COMPOSE_PROFILES=tournaments/' .env
docker compose up -d
docker compose stop courthive-hiveid courthive-server postgres redis
```

The last command names the tournament parts; `docker compose up -d` on its own
leaves them running. Their data is kept. To turn tournaments back on later,
remove the `#` and run `docker compose up -d`.

## Trying it locally, without a domain

You can try the app on your own computer, or on a spare machine, before
renting anything. It runs the same way, at the address `https://localhost`,
with a certificate made on the spot by Caddy instead of a real one. Use it to
look around, to practise a restore, or to test an upgrade. Do not use this on
a real server.

You need:

- Docker: **Docker Desktop** on Windows or Mac
  (<https://docs.docker.com/desktop/>), or Docker Engine on Linux (step 4).
  On Windows, run the commands below in a WSL (Ubuntu) terminal, which Docker
  Desktop sets up.
- Nothing else on the computer using ports 80 and 443 (if something is, see
  "If ports 80 and 443 are busy" below).
- An internet connection: set-up reads your league from the BHPLTA website.

In a terminal (type the latest version number in place of `X.Y.Z`):

```
mkdir -p ~/jim-dot-tennis-trial && cd ~/jim-dot-tennis-trial
VERSION=X.Y.Z
for f in docker-compose.yml .env.example Caddyfile init-tournaments.sh; do curl -fsSLO "https://raw.githubusercontent.com/jameshartt/jim-dot-tennis/v$VERSION/deploy/$f"; done
chmod +x init-tournaments.sh
cp .env.example .env
chmod 600 .env
sed -i.bak "s|^DOMAIN=.*|DOMAIN=localhost|; s|^PUBLIC_URL=.*|PUBLIC_URL=https://localhost|" .env
grep -E '^(DOMAIN|PUBLIC_URL)=' .env
```

(The `.bak` makes the command work on a Mac too; it leaves a copy of the old
file as `.env.bak`, which you can delete.) You should see
`DOMAIN=localhost` and `PUBLIC_URL=https://localhost`.

If this computer already uses ports 80 or 443, or already runs another copy of
the app, do "If ports 80 and 443 are busy" below now, before starting.
**This matters:** without it, Docker treats the trial as the other copy (they
share the same name), replaces its containers and uses its data.

Start it and read the set-up code, exactly as in steps 7 and 8:

```
docker compose up -d
docker compose logs --no-log-prefix app | grep "SETUP CODE" | tail -1
```

Open `https://localhost`. The browser warns that the connection is not private,
because it does not know Caddy's home-made certificate:

- Chrome or Edge: "Your connection is not private". Click **Advanced**, then
  **Proceed to localhost (unsafe)**.
- Firefox: "Warning: Potential Security Risk Ahead". Click **Advanced**, then
  **Accept the Risk and Continue**.
- Safari: "This Connection Is Not Private". Click **Show Details**, then
  **visit this website**.

Then carry on with step 9. Choose a password at the "Create your login" step:
browsers may refuse passkeys on a site whose certificate they do not trust.

At "Choose your club" a trial can also use **Explore with demo data** (under
"Just looking?"): a made-up club, players and results, with a "Demo data"
banner on every page. It needs nothing from the BHPLTA website, and it can be
cleared later to choose a real club.

To stop the warnings (optional), copy Caddy's certificate out and add it to
your computer's or browser's trusted certificates:

```
docker compose cp caddy:/data/caddy/pki/authorities/local/root.crt ./caddy-local-root.crt
```

Tournaments work locally too: follow "Optional: turning on tournaments" in
this folder. On a Mac or an ARM computer the tournament containers will not
start (they are x86-64 only).

### If ports 80 and 443 are busy

If `docker compose up -d` fails with `port is already allocated` or
`address already in use`, or this computer already runs another copy of the
app, run the trial on port 8443 under its own name instead. In the trial
folder, add two settings and a small extra file that Docker Compose reads
automatically:

```
echo COMPOSE_PROJECT_NAME=jdt-trial >> .env
sed -i.bak "s|^PUBLIC_URL=.*|PUBLIC_URL=https://localhost:8443|" .env
cat > docker-compose.override.yml <<'EOF'
# Only for trying the app on a computer that already runs another copy.
services:
  app: { container_name: !reset null }
  caddy:
    container_name: !reset null
    ports: !override
      - "127.0.0.1:8443:443"
  redis: { container_name: !reset null }
  postgres: { container_name: !reset null }
  courthive-pre-upgrade: { container_name: !reset null }
  courthive-server: { container_name: !reset null }
  courthive-hiveid: { container_name: !reset null }
  courthive-hash-passwords: { container_name: !reset null }
  courthive-service-account: { container_name: !reset null }
  tmx-content: { container_name: !reset null }
  public-content: { container_name: !reset null }
volumes:
  tennis-data: { name: jdt-trial-tennis-data }
  caddy-data: { name: jdt-trial-caddy-data }
  caddy-config: { name: jdt-trial-caddy-config }
  courthive-pg-data: { name: jdt-trial-pg-data }
  redis-data: { name: jdt-trial-redis-data }
  courthive-archives: { name: jdt-trial-archives }
  tmx-files: { name: jdt-trial-tmx-files }
  public-files: { name: jdt-trial-public-files }
EOF
docker compose config --services
```

The last command should list `app` and `caddy` (and the tournament parts if
you turned them on) with no error. If it says `!reset` or `!override` is not
understood, your Docker Compose is too old: update Docker Desktop.

If you already tried to start the trial with the ordinary settings on a
computer with no other copy of the app, and it failed only because the ports
were busy, run `docker compose down` (without `-v`) before this. Then start as
above and open `https://localhost:8443`. The containers are then called `jdt-trial-app-1`,
`jdt-trial-caddy-1` and so on; every `docker compose` command in these guides
still works from the trial folder.

### Removing the trial

In the trial folder:

```
docker compose down -v
```

This deletes the trial's containers **and all its data**. Never run it with
`-v` on your real server, and on a computer that runs another copy of the app
only from a trial folder whose `.env` has the `COMPOSE_PROJECT_NAME=jdt-trial`
line (check with `grep COMPOSE_PROJECT_NAME .env`).
