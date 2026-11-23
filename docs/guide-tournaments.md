# Club championships and tournaments

This guide is a short path from switching tournaments on to publishing a club championship draw. The tournament software is built by CourtHive. For depth on drawing, scheduling and scoring, use CourtHive's own documentation, which you can find from [courthive.com](https://courthive.com). This guide covers the part that is specific to your club's app.

Related guides: the [administrator guide](guide-admin.md), and the installation guide (`install.md`), which explains how the person who looks after the server switches tournaments on at the server.

## Before you start

You need three things.

1. The person who looks after the server has switched the tournament software on. See the installation guide (`install.md`).
2. You are an administrator, or an administrator has given you tournament access.
3. Tournaments is switched on in the app. On the dashboard, choose **Club Settings**, find **Features**, tick **Tournaments** and press **Save settings**.

## Set the club up for tournaments

An administrator does this once.

If tournaments were switched on from **Club Settings**, the club was set up at the same moment. The **Tournament set-up** panel then already says the club is set up and offers **Check the set-up again**. In that case there is nothing to do here: go on to the next section.

1. On the dashboard, choose **Manage Tournaments**.
2. Find the **Tournament set-up** panel.
3. Check the **Club's tournament code**. It is made from your club's name. It cannot be changed afterwards, so check it first.
4. Press **Set up tournaments**.
5. The panel lists each step and says what happened. If a step fails, press **Try again**. Nothing is left half done.

When it says the club is set up, you are ready.

## Give people access

Administrators can always manage every tournament. For anyone else:

1. On the dashboard, choose **Manage Users**.
2. For a new person, use **Create New User**. Choose **Tournaments only** as the role if they run tournaments and nothing else. Press **Create user and invite link** and pass on the link.
3. In the **Tournaments** column, choose what each person can do: **No tournaments**, **Own tournaments** (those they run themselves and any they have been given) or **All tournaments**.

A captain has no tournament access until you give it here. People with a Tournaments only account land on a tournament home page with one button.

## Sets of tournaments

Most clubs need only one set of tournaments and can skip this section.

A club can keep separate sets of tournaments. For example, one for the club's own championships and one for a league cup. Each set has its own tournaments and its own list of people who can manage it.

**Add a set.**

1. On the **Manage Tournaments** page, go to **Tournament set-up**, then **Add another set of tournaments**.
2. Type a **Name**. This is what people see, for example "Parks Cup".
3. Type a **Code**, 2 to 12 letters or digits. It cannot be changed later.
4. Press **Add**.

**Rename a set.** Under **Sets of tournaments**, change **Name people see** and press **Save name**. The code stays the same.

**Choose who can manage each set.** A new set is given to nobody except administrators. On **Manage Users**, a tick box list headed **Can manage tournaments for** appears beside each person who has tournament access, once there is more than one set. Tick the sets for each person and press **Save**. Someone with none ticked gets the club's own set only.

**What people see.** Someone with more than one set is asked **Which tournaments do you want to manage?** on their tournament home page, with the sets listed by name. They choose one and go in. To manage the other set later, they come back to that page and choose it.

## Create the club championship

Open tournament management. Administrators and captains with tournament access can use **Open Tournament Management** on the dashboard. Tournaments only accounts use the button on their home page.

The usual path, in the tournament software, is:

1. **Create a tournament.** Give it a name and dates, for example "Club Championships".
2. **Add players.** Enter the entrants. Ask them to sign up, or add them yourself.
3. **Add an event.** An event is one competition, such as Men's Singles or Mixed Doubles. Choose the format.
4. **Generate the draw.** The software builds the draw from the entries. Seed the strongest players first if you want to.
5. **Publish.** Publishing makes the draw visible on the public pages, so members can see it.

The exact buttons change as CourtHive improves its software. Their documentation is the best guide to each screen.

## Show it on the club's pages

1. Back in the app, go to **Manage Tournaments**.
2. Press **Sync from CourtHive** under the right set. The app counts new, updated and unchanged tournaments.
3. In the list, check the **Visibility** button. **Visible** tournaments appear on the club's public page. Press the button to switch between **Visible** and **Hidden**.
4. Use **Edit** to change the name, dates or description.

## If something goes wrong

- If tournament management does not open, try again in a minute. If it keeps happening, tell an administrator.
- If the Tournament set-up panel says the bundled tournament software is not switched on, ask the person who looks after the server. See the installation guide (`install.md`).
- For anything else, see the troubleshooting guide (`troubleshooting.md`).
