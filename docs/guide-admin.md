# Administrator guide

This guide is for the people who look after the club's account of the app: the administrators. It covers setting up your first season, inviting captains and other people, giving players their links, club settings, starting a new season and importing match cards.

You do not need to be technical. If you are the person who installs and upgrades the app on a server, see the installation guide (`install.md`) as well.

Related guides: the [captain guide](guide-captain.md) for the weekly routine, the [player sheet](guide-player.md) to send round, and the [tournaments guide](guide-tournaments.md) for club championships.

## Sign in for the first time

The first time the app starts, it asks you to set up. You choose your club and create your own login. That login becomes the first administrator. Nobody else has a way in.

1. Open the web address of the app.
2. Enter the setup code the person who installed the app gives you, then follow the steps.
3. At "Create your login", choose a username and a password. Or choose "Create my login, then add a passkey" to sign in with your fingerprint, face or screen lock instead of typing a password.
4. When you see "You're all set", the app has read your league from the BHPLTA website: divisions, teams, weeks and fixtures.

After that you land on the **Admin Dashboard**. Every page below starts from it. Each page also has a "Help with this page" link that opens the matching part of this guide.

Use the **Sign-in methods** button at the top of the dashboard to add a passkey or change your password at any time.

## Check your first season

Setup creates the first season for you. A few minutes of checking now saves questions later.

1. On the dashboard, choose **Manage Seasons**.
2. Check that your season shows **ACTIVE**. If it does not, press **Set Active** beside it.
3. Press **Setup** beside the season. You see each division, with its teams.
4. Check that your club's teams are in the right divisions. To move a team, choose the division next to it and press **Move**.
5. Press **Review Away Teams** to check the other clubs' teams in your divisions.

If something looks wrong, ask your league secretary before changing it. The data comes from the BHPLTA website.

## Add players

Players do not have accounts. They get a private link instead (see the next section). You add each player once.

The app cannot bring your players in for you. Setup, the season set-up from BHPLTA and **Club Data Import** read clubs, divisions, teams and fixtures, but not players. **Match Card Import** only matches the names on a match card to players you have already added. So the quickest way for a new club is to add your players by hand, as below, before you import any match cards. It takes about a minute for each player.

Spell each name as the BHPLTA match cards do. The match card import finds players by name, and a name it cannot match is listed as unmatched.

1. On the dashboard, choose **Manage Players**.
2. Press **+ Add New Player**.
3. Enter the first name, last name and gender, then press **Create Player**.
4. Go to **Manage Teams**, choose a team, press **Add Players**, tick the players for that squad and confirm.

Once your players are added, a match card import adds any of your club's players who played for one of your teams to that team's squad for the season. You can still add players to squads by hand before then, so that captains can pick them.

To change a name or hide a player who has left, open the player from the players list. The page has **Update Player**, and **Deactivate Player** (with **Reactivate** if they come back). A deactivated player keeps their history.

## Give players their links

Each player has a personal link. It opens that player's own availability page. No sign-in is needed.

1. Go to **Manage Players**.
2. Find the player. In the last column, press **Generate** the first time. It makes the link and copies it.
3. After that, the button says **Copy**. Press it to copy the link again.
4. Send the link to the player in a private message.
5. Send the [player sheet](guide-player.md) with it. It explains what the link does in a page.

The players list also shows who has set their availability for next week, and who has turned on notifications.

To nudge people who have not replied, press **Remind All** on the players page. You can also remind just one team. Reminders reach players who have turned on notifications.

## Invite a captain

A captain runs the club's teams. A captain can see and work with every team. Captains need an account.

1. On the dashboard, choose **Manage Users**.
2. Under "Create New User", type a **Username**.
3. Set **Role** to `captain`.
4. Optional: choose the captain's player under **Linked player (optional)**. The planning dashboard then marks their own teams with a star.
5. Press **Create user and invite link**.
6. Copy the invite link shown at the top of the page. It is shown only once, and the app does not email it. Give it to the captain yourself, in person or in a private message.
7. The captain opens the link, chooses a passkey or a password, and is signed in.

An invite link works once and lasts 48 hours. If it was sent to the wrong person, press **Cancel link** under "Links not used yet". If it runs out, press **New invite link** beside the person.

To make someone a team captain on a team's page, see "Name the team captains" below. That is separate from having an account.

### Name the team captains

1. Go to **Manage Teams** and open the team.
2. Under "Team Captains", press **Add Captain**.
3. Choose the player and the role, **Team Captain** or **Day Captain**, then confirm.

## Invite other people

Use the same **Create New User** form for anyone else.

- **admin**: can do everything a captain can, and also runs the site. Give this to a few trusted people. There must always be at least one administrator who can sign in.
- **captain**: runs the teams.
- **Tournaments only**: for people who run the club's tournaments and nothing else. They go straight to the tournament home page. See the [tournaments guide](guide-tournaments.md).

If someone loses their way in, press **Recovery link** beside them. Nothing changes for them until they use the link, which lasts one hour. They then choose a new passkey or password and are signed out everywhere else.

## The users page

The users page lists everyone with an account.

| Column | What it shows |
|--------|---------------|
| Username | The sign-in name. Your own row says "(you)". |
| Role | Change it from the list. Changes apply at once. |
| Tournaments | No, own or all tournaments. See the [tournaments guide](guide-tournaments.md). |
| Linked player | Connects the account to a player record. |
| Status | Active or Inactive. Press **Deactivate** to stop an account signing in. |
| Signs in with | The number of passkeys, a password, or "Invite not accepted". |
| Last Login | The last time they signed in. |

Every change is written to the **Account Audit Log**, which you can read from the dashboard. The **View Sessions** page shows who is signed in, with **Revoke** and **Revoke All** to sign people out.

## Club settings

On the dashboard, choose **Club Settings**. Press **Save settings** after changing the top sections.

- **Club identity**: the display name, logo and two colours. The logo is also used as the app icon on phones.
- **BHPLTA match cards**: your club code (the club scores password), BHPLTA club name and BHPLTA club id. Match-card import needs these. After saving the club code, press **Look up from BHPLTA** to fill in the name and id for you. Your league secretary can tell you the club code.
- **This instance**: the **Public URL**, which is the address people use, starting with `https://`. Links in messages, calendar files and sign-in use it.
- **Features**: switch **Tournaments**, **Club Wrapped** and **My Tennis** on or off. A feature that is off disappears from the menus.
- **Sign-in for administrators**: **Require passkeys for administrators** stops administrators signing in with a password. Each administrator must add a passkey first.
- **Club Wrapped access code**: the code members type to open the end-of-season review. Share it however you like.

## Start next season

Do this once a year, before the new season begins.

1. Go to **Manage Seasons**.
2. Press **Create Season**. Enter the name (for example "2027 Season"), the year, the start and end dates and the number of weeks (usually 18). Press **Create Season**.
3. Press **Setup** beside the new season, then **Import Season Data**.
4. Leave **Copy Teams with Players & Captains** ticked. This brings your squads and captains across from last season.
5. Leave the BHPLTA fixtures address as it is. The divisions, teams and fixtures are read from the BHPLTA website. BHPLTA only lists upcoming fixtures, so do this before the season starts. Or choose a fixture file made with the fixture generator instead.
6. Press **Import Season Data**.
7. Press **Review Away Teams** and check the other clubs' teams.
8. Back on **Manage Seasons**, press **Set Active** beside the new season.

If there is no earlier season to copy, press **Start a new season from BHPLTA** on the seasons page instead. It creates the season, weeks, divisions, teams and fixtures in one go, and any club the app does not know yet.

You can delete a season that is not active. This removes all its data, so the app asks you to confirm.

## Import match cards

Match results come from the BHPLTA website. You or a captain can import them week by week.

1. On the dashboard, choose **Match Card Import**.
2. Choose the **Week** and check the **Year**. The club name, id and code are filled in from Club Settings. If they are empty, set them there first.
3. Tick **Dry Run Mode** the first time. It shows what would happen without saving anything.
4. Press **Start Import** and read the results on the right.
5. If it looks right, untick **Dry Run Mode** and press **Start Import** again.

Two other boxes are ticked by default. **Clear Existing Matchups** replaces matchups already stored with the ones from BHPLTA, which is useful when a card was corrected. **Clear Pairings on Unplayed Fixtures** clears the selected players for fixtures that passed their date with no result. Those fixtures are marked **Awaiting Reschedule**, and a captain sets a new date.

## Other admin pages

- **Preferred Name Approvals**: players can ask to be called something else on team sheets. Approve or reject each request here. Captains can do this too.
- **Manage Clubs**: other clubs in the league and their teams. Administrators can edit them. Captains can look.
- **Club Data Import**: reads every club's details from the BHPLTA website. Use it when venue details change.
- **Backups**: shows when the last backup was made and lets you download one. See the backup guide (`backup-and-restore.md`).
- **Points Table** and **Selection Overview**: the same pages captains use.

## What administrators and captains can do

An administrator can do everything a captain can, and also runs the site. A captain can see and work with every team, not only their own. The club works as one squad.

| Area | Captain | Administrator |
|------|:-------:|:-------------:|
| Dashboard (the page you land on after signing in) | Yes | Yes |
| Captain planning dashboard, availability matrix, "My Teams" | Yes | Yes |
| Linking your own account to your player record ("I am...") | Yes | Yes |
| Player availability and reminders to give availability | Yes | Yes |
| Team selection, matchups, day captains | Yes | Yes |
| Fixtures, rescheduling, fixture notes, weather, calendar files | Yes | Yes |
| Result entry and match card import from BHPLTA | Yes | Yes |
| Push notifications: "notify selected players" and "remind availability" | Yes | Yes |
| Players: add, edit, deactivate, reactivate, availability links | Yes | Yes |
| Preferred name requests from players | Yes | Yes |
| Points table, standings and selection overview | Yes | Yes |
| Captain notes | Yes | Yes |
| Teams: view every team, add players to a squad | Yes | Yes |
| Other clubs and their teams: view | Yes | Yes |
| Teams: create a team, assign or remove team captains, mark a team active or inactive | No | Yes |
| Other clubs and their teams: create, edit, delete | No | Yes |
| Users: create accounts, invite and recovery links, change roles and tournament access, link an account to a player | No | Yes |
| Sessions: see who is signed in, sign people out | No | Yes |
| Account audit log | No | Yes |
| Club settings and feature switches (My Tennis, Tournaments, Club Wrapped) | No | Yes |
| BHPLTA settings and club data import | No | Yes |
| Seasons: create, copy, set up, set active, delete, start from BHPLTA | No | Yes |
| Divisions and the division review | No | Yes |
| Tournaments (CourtHive): the club's tournament list and sets of tournaments | No | Yes |
| Tournament set-up: setting the club up on the tournament server, and running it again | No | Yes |
| Tournament management | If given access | Yes, always all |
| Season Wrapped (the club's season review page) | No | Yes |
| Demo data: load and clear | No | Yes |
| Passkey security warnings: clear a blocked passkey | No | Yes |
| Push notifications: test message to every subscriber, reset the push keys | No | Yes |
| Backups: see when the last one was made, download one (asks for your password or a passkey) | No | Yes |

A third kind of account, **Tournaments only**, has none of the league pages. It has its tournament home page and its own sign-in methods.

If a captain opens a page that is only for administrators, they see a page saying so, with a link back to the dashboard. Links to those pages are not shown to captains.

Everyone with an account can manage their own sign-in methods on the **Sign-in methods** page. Changing them asks you to confirm it is you first, with your current password or a passkey you already have.

## Getting help

If something does not work as described here, see the troubleshooting guide (`troubleshooting.md`). For sign-in problems, see the accounts and recovery guide (`accounts-and-recovery.md`).
