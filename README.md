# jim-dot-tennis

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Run your club's side of the Brighton & Hove Parks League from one place: who is available, who plays, where and when, and what the results were. Your club hosts its own copy, on its own server.

## Who it is for

Clubs in the BHPLTA (Brighton & Hove Parks League). It was built at St Ann's Tennis Club and is packaged so any club in the league can run it. A volunteer with modest technical confidence can install it by following the guide.

## What you get

- **Availability without accounts.** Each player has a private link and says when they are free. Captains see everyone at a glance.
- **Team selection.** Captains pick teams from who is available, and every captain can see the whole squad.
- **Fixtures and calendars.** Fixtures with venues, maps and directions, and calendar feeds players can subscribe to.
- **Results and standings.** Match cards imported from the BHPLTA website, league tables and match history.
- **Your own club championships.** An optional, built-in tournament system for draws, schedules, live scores and public tournament pages. You switch it on when you want it.
- **Sign-in that is hard to get wrong.** Captains and administrators use passkeys (fingerprint, face or device PIN). A password is an optional fallback. There is no email to set up.
- **Backups built in.** The app backs itself up every day and before every upgrade.
- **Help in the app.** Guides for administrators, captains and players are part of the app: open `/help` on your site, or press **Help with this page** on any admin page.

## Your data stays with your club

Each club runs its own copy and nobody else has access to it, including the author. A new copy has no accounts and no shared passwords, and it sends nothing back to anyone. [What your copy connects to](docs/outbound_connections.md) lists every outside connection, and [Privacy and data](docs/privacy-and-data.md) explains what is stored.

## Install

You need a small server (1 GB of memory, or 2 GB with tournaments), Docker and a domain name. Follow [docs/install.md](docs/install.md). It takes the published release image; nothing is built on your server.

## Keeping it going

[Upgrade](docs/upgrade.md), [back up and restore](docs/backup-and-restore.md), [troubleshooting](docs/troubleshooting.md) and [accounts and recovery](docs/accounts-and-recovery.md) are all in the [documentation index](docs/README.md). See [CHANGELOG.md](CHANGELOG.md) for what changed in each version.

## For developers

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, tests and code style. The project is open source under the MIT licence ([LICENSE](LICENSE)). Releases are described in [RELEASING.md](RELEASING.md).

## Credits

Built by [James Hartt](https://github.com/jameshartt) at St Ann's Tennis Club. Tournament management is by [CourtHive](https://courthive.com/) (Charles Allen), bundled under its MIT licence. See [ACKNOWLEDGEMENTS.md](ACKNOWLEDGEMENTS.md).
