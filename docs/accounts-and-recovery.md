# Accounts and recovery from the server

Most account work is done in the app, on the users page, by an administrator
(see the [administrator's guide](guide-admin.md)). This guide is for when that
is not possible: nobody can sign in as an administrator, or you want to do it
from the server.

The app's own program has commands for this. You run them on the server, in the
install folder (see [Installing](install.md)), while the app keeps running:

```
cd ~/jim-dot-tennis
docker compose exec app ./jim-dot-tennis user list
```

`docker exec -it jim-dot-tennis ./jim-dot-tennis user list` does the same; the
app's own messages use that form.

To see every command with a short explanation:

```
docker compose exec app ./jim-dot-tennis help
```

## Things to know first

- **No command ever asks for, shows or sets a password.** To get someone in, a
  command prints a single-use **link**. Whoever opens the link sets up their
  own passkey or password. There is no email: pass the link on yourself,
  privately (in person, by phone or in a private message, never in a group
  chat). Whoever opens it first gets the account.
- A new link for an account cancels that account's earlier unused links.
- Links start with your site's address (`PUBLIC_URL` in `.env`).
- Anyone who can run these commands already controls the server and the app's
  data. The commands make recovery easier; they give nobody any extra power.
  Nobody outside your club, including the people who wrote the app, has a way
  in.
- Each command records what it did (time, command, account) in the account
  audit log that administrators can read, and prints a line starting `audit:`.
  No password, code or link is ever recorded.
- If the app was upgraded but has not started since, the commands refuse until
  it has.

## Who is there?

```
docker compose exec app ./jim-dot-tennis user list
```

You should see a table like this:

```
USERNAME      ROLE              TOURNAMENTS          CAN SIGN IN  LAST SIGNED IN     SIGNED IN ON  NOTE
alex.captain  captain           own                  yes          never              0 device(s)   no way to sign in yet (invite not used)
robin.td      Tournaments only  all                  yes          never              0 device(s)   no way to sign in yet (invite not used)
sam.jones     admin             all (administrator)  yes          23 Nov 2026 08:33  1 device(s)

3 account(s), 1 active administrator(s).
```

The kinds of account (roles) are:

- `admin` (administrator): runs the site for the club, and does everything a
  captain does;
- `captain`: runs every team;
- `tournaments` (shown as "Tournaments only"): runs the club's tournaments and
  sees no league pages.

[Roles](roles.md) says exactly what each can do.

## Someone is locked out (lost phone, forgotten password)

```
docker compose exec app ./jim-dot-tennis user reset <username>
```

You should see `Made a recovery link for "<username>" ... Nothing has changed
for the account yet.`, then the link, which works once, for **1 hour**. Pass it
on. When they open it they set a new passkey or password and are signed in, and
the account is signed out everywhere else. On that page they can also tick a
box to remove their old passkeys and password (do this when a phone was lost or
stolen).

Making the link changes nothing until it is used, so it is safe to make one by
mistake.

## Every administrator is locked out

This works even for the last administrator.

1. See who the administrators are:

   ```
   docker compose exec app ./jim-dot-tennis user list
   ```

2. Make a recovery link for one of them:

   ```
   docker compose exec app ./jim-dot-tennis user reset <username>
   ```

3. Open the link in your browser, set a passkey or password, and you are signed
   in as that administrator.

If the account is disabled (`CAN SIGN IN` says `no`), enable it first, or the
link will not work:

```
docker compose exec app ./jim-dot-tennis user enable <username>
```

If the club requires passkeys for administrators and you cannot make one on
the link page, switch that requirement off first (next section).

If no administrator account is left at all, make one of the existing accounts
an administrator, or create a new one:

```
docker compose exec app ./jim-dot-tennis user role <username> admin
docker compose exec app ./jim-dot-tennis user create <new-username> --role admin
```

`user create` prints an **invite link** (it works once, for **48 hours**).

## "Require passkeys for administrators"

An administrator can require that administrators sign in with a passkey only
(Club Settings). If that locks you out:

```
docker compose exec app ./jim-dot-tennis admin-passkeys status
docker compose exec app ./jim-dot-tennis admin-passkeys off
```

`status` says whether it is required and lists administrators with no working
passkey. `off` stops requiring it at once (no restart): administrators can sign
in with a password again. It says
`Administrators may sign in with a passkey or a password.`

To require it again once every administrator has a passkey:

```
docker compose exec app ./jim-dot-tennis admin-passkeys on
```

It is refused, naming them, while any active administrator has no working
passkey.

## Making and changing accounts

| What | Command |
|---|---|
| New account (prints an invite link) | `docker compose exec app ./jim-dot-tennis user create <username> --role captain` |
| Change someone's role | `docker compose exec app ./jim-dot-tennis user role <username> <admin\|captain>` |
| Stop someone signing in, and sign them out everywhere | `docker compose exec app ./jim-dot-tennis user disable <username>` |
| Let a disabled account sign in again | `docker compose exec app ./jim-dot-tennis user enable <username>` |
| Sign someone out on every device | `docker compose exec app ./jim-dot-tennis user sessions revoke <username>` |

Usernames are letters and numbers, plus `.`, `_`, `-` or `@`, with no spaces.

The app protects you from locking the club out: disabling or demoting the last
administrator who can sign in is refused with `Not done: this is the last
administrator who can sign in: ...`. Creating an administrator, `user reset`
and `admin-passkeys off` are never refused, so there is always a way back in
from the server.

## Tournament access

With tournaments on (see [Installing](install.md), "Optional: turning on
tournaments"), every account also has a tournament access setting:

- `none`: no tournament management;
- `own`: the tournaments they run themselves, and any they are given;
- `all`: every tournament the club runs.

Administrators always have all. A captain has none unless you give it. A
Tournaments only account always has `own` or `all`; to stop it, disable the
account.

Create someone who only runs tournaments:

```
docker compose exec app ./jim-dot-tennis user create <username> --role tournaments --tournaments own
```

Give a captain tournament access, or change anyone's (except an
administrator's):

```
docker compose exec app ./jim-dot-tennis user tournaments <username> all
```

It says `Changed the tournament access of "<username>" from ... to all.`

### Choosing which sets of tournaments a person can manage

A club can keep more than one set of tournaments, for example one for the
club's own tournaments and one for a league cup. Most clubs need only one, made
when tournaments are switched on. Each set has a short **code** (2 to 12
letters or digits, fixed once made) and a **name** people see when they choose
which tournaments to manage.

See the sets and their codes:

```
docker compose exec app ./jim-dot-tennis tournaments status
```

The end of its output lists them:

```
Sets of tournaments:
  HOVEPARK     Hove Park Tennis Club (the club's own)
  PARKSCUP     Parks Cup
```

Add another set. The command's name uses the tournament software's own word
for a set; type a code and the name people will see:

```
docker compose exec app ./jim-dot-tennis tournaments add-provider PARKSCUP "Parks Cup"
```

It lists its steps and ends `Done. Nobody but administrators has been given it
yet.` (An administrator can do the same on the Tournaments page, "Add another
set of tournaments".) If the tournament software already has a set with that
code, it is taken on as it is, with its tournaments.

Choose which sets someone can manage, with the codes separated by commas
(this replaces their earlier choice):

```
docker compose exec app ./jim-dot-tennis user tournaments <username> own --providers HOVEPARK,PARKSCUP
```

It says `"<username>" can now manage these sets of tournaments: HOVEPARK,PARKSCUP.`
The same can be done when creating an account, by adding
`--providers HOVEPARK,PARKSCUP` to `user create`.

- Administrators always have every set.
- Someone with nothing chosen has the club's own set only.
- A new set is given to nobody but administrators until you choose people.
- Someone with more than one set is asked "Which tournaments do you want to
  manage?" when they open tournament management.

On the users page in the app the same choice is the "Can manage tournaments
for" tick boxes.

### Tournament set-up from the server

Switching Tournaments on in Club Settings sets the club up on the tournament
software. To run that set-up again from the server (it is always safe: what is
already there is left alone, and a set-up that stopped part-way is finished):

```
docker compose exec app ./jim-dot-tennis tournaments provision
```

It lists each step with `Made`, `Already there` or `Done`, and ends with
`Done.`. If a step fails it names it, for example
`Stopped at: Connect this app to the tournament server.`, with the reason: see
[Troubleshooting](troubleshooting.md).

## The first-run set-up code

Only while the app has no administrator at all (a brand new install):

```
docker compose exec app ./jim-dot-tennis setup code
```

It prints a new code and cancels any earlier one, including the one in the
log. Once an administrator exists it answers
`No setup code is needed: this app already has an administrator account.` and
points you to `user reset`.
