# Privacy and your club's data

Your club runs its own copy of the app, on a server the club rents, at an
address the club owns. This page says what personal data that copy holds, who
is responsible for it, where it connects to, and who can see it. At the end is
a short notice your club can adapt for its members.

This is a plain description of what the software does, not legal advice. If
your club is unsure about its duties under UK data protection law (UK GDPR and
the Data Protection Act 2018), the Information Commissioner's Office (ICO) has
guidance for small organisations and clubs at <https://ico.org.uk>.

## Who is responsible

**Your club is the "controller"** of the personal data in its copy of the app:
the club decides why the data is kept and how it is used. The people who wrote
the app do not run your copy, cannot see into it and do not process any of its
data, so they are not your "processor" either.

If the club rents its server from a hosting company, that company stores the
server's disk for the club and may be a processor: check its terms (most have
a data processing agreement).

## Who has access

Only your club. In practice:

- **People with accounts**, through the app, as far as their role allows
  (see [Roles](roles.md)): administrators see everything; captains see every
  team, its players and their availability; Tournaments only accounts see the
  tournament software only.
- **Players**, through their own private link, see and change their own
  availability and profile.
- **Whoever can log in to the server** (the person who installed it, and
  anyone else given the server's password or key). They can read the whole
  database and every backup. Keep that list short.
- **The hosting company**, which could in principle read the server's disk.

**Nobody else.** A new installation has no accounts, no built-in passwords and
no shared keys: every secret is made on your server when it is installed. The
app sends nothing to the people who wrote it: no usage statistics, no error
reports, no update checks. The software's author has no account on, and no way
into, your club's copy.

## What personal data it holds

### About players

- Name, and a preferred name if one was requested.
- Gender (used for mixed and same-gender doubles).
- Which club, teams and divisions they play in.
- Their availability for fixtures, and the fixtures they were picked for.
- Match results and scores, imported from the BHPLTA match cards or entered by
  captains.
- Whether they appear in public reports (a privacy setting per player).
- If "My Tennis" is switched on: the answers they choose to give about their
  tennis (how long they have played, playing style, partner preferences and
  similar).
- Notes captains write about players and partnerships.
- If they turn on phone or browser notifications: an address for their
  browser's notification service.

Players have no email address or phone number in the app. Most player names
and results come from the public BHPLTA website.

### About people with accounts (administrators, captains, tournament organisers)

- Username, role, and the player record it is linked to, if any.
- A one-way hash of their password (never the password itself), and the
  public part of each passkey.
- Signed-in sessions, with the browser's description and network address.
- Sign-in attempts (username and network address), kept for 90 days.
- An audit log of account actions: who did what to which account, and when.
  It never holds passwords, links or codes, and it is kept permanently: the
  app does not delete it.

### With tournaments on

The tournament software holds what organisers enter: entrants' names and the
details a tournament asks for, draws, schedules and results. The app links each
person's account to a tournament account under a made-up address (ending
`.invalid`), not their email.

### Copies

Backups hold all of the above. The app keeps 14 daily backups on the server by
default, and you keep copies elsewhere ([Backups and
restoring](backup-and-restore.md)). Something deleted in the app stays in
backups until they are deleted too.

The app has no button that erases a player's record completely: a player can
be marked inactive. If someone asks for their data to be erased, the person
who runs the server will need to arrange it.

## Every connection the app makes

### From the server

| To | When | What is sent |
|---|---|---|
| The BHPLTA website (`www.bhplta.co.uk`) | When an administrator imports a season, match cards or club details; at first set-up; when someone opens the fixture generator | Ordinary page requests. For match cards and club look-up, your club's BHPLTA club code and name, as the BHPLTA website itself asks for them. Every request names this software, its version and its public source code; it never pretends to be a web browser. No personal data. |
| A fixtures page address an administrator types in | When that administrator imports a season from it | A page request |
| Open-Meteo (`api.open-meteo.com`), a free weather service | When an administrator opens a fixture page within the forecast window | The venue's location and the dates. No account, no names. |
| Browser notification services: Google (`fcm.googleapis.com`, `android.googleapis.com`), Mozilla (`push.services.mozilla.com`), Apple (`push.apple.com`), Microsoft (`notify.windows.com`) | When a notification is sent to someone who turned notifications on | The notification, encrypted so only that person's browser can read it, and your site's address |
| Let's Encrypt (or ZeroSSL), through Caddy | Getting and renewing the site's security certificate | Your domain name |
| Docker Hub (`docker.io`) and the GitHub container registry (`ghcr.io`) | Only when you run `docker compose pull` to install or upgrade | Requests for the software |

With tournaments on, the tournament parts talk only to each other and to the
app, inside the server. They make no connections outside it. The app uses them
to set your club up, to give people tournament access (it sends their
usernames), and for backups of the tournament data.

No part of the app downloads anything when it starts.

There are no others: no analytics, telemetry, error reporting, crash uploads,
licence checks, update checks or remote logging. The app sends no email and no
text messages.

### From visitors' browsers

The app's pages load everything (scripts, styles, fonts) from your own site,
with one exception:

| To | When |
|---|---|
| OpenStreetMap (`www.openstreetmap.org`) | A venue page shows a map. The visitor's browser fetches the map, sending its network address and the map area. |

Links to other sites (directions in Google or Apple Maps, sharing on WhatsApp,
Wikipedia, the source code on GitHub) are only followed if someone clicks them.

With tournaments on, the tournament pages also load everything from your own
site. They contact other sites only when an organiser chooses to: a screen
font other than the default (Google Fonts), a map, a linked Google Sheet, or an
image or link the organiser typed in.

The full technical list, kept in step with the code by an automatic test, is
in [outbound_connections.md](outbound_connections.md).

## A notice your club can adapt

Put this where members will see it (the club website, the membership form, or
both). Replace the parts in square brackets.

> **How [Club name] uses your information in our tennis app**
>
> [Club name] runs its own tennis app at [https://tennis.example.org] to
> organise our league teams[, and our tournaments]. [Club name] is responsible
> for the information in it.
>
> **What we keep:** your name and gender; the teams you play for; your
> availability; the matches you are picked for and their results (most of which
> also appear on the BHPLTA website)[; and anything you choose to tell us in
> "My Tennis"]. If you have a login, we also keep your username and records of
> when you signed in. [For tournaments, we keep your entries and results.]
>
> **Why:** to pick teams, arrange fixtures and record results for the Brighton
> and Hove Parks Tennis League[ and to run club tournaments]. Our lawful basis is
> [legitimate interests: running the club's tennis].
>
> **Who can see it:** our captains and the committee members who run the app.
> Players can see their own availability through their personal link. The app
> runs on a server we rent from [hosting company]. Nobody else, including the
> people who wrote the software, has access. We do not sell or share your
> information.
>
> **How long:** while you play for the club, and [one season] after you stop.
> Backups are kept for [two weeks].
>
> **Your rights:** you can ask to see the information we hold about you, to
> correct it, or to have it removed. Contact [name, role] at [email address].
> If you are not happy with our answer you can complain to the Information
> Commissioner's Office (ico.org.uk).
