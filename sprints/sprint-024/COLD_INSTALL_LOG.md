# Cold install test (WI-151)

A tester with no knowledge of the project (a Sonnet 5.5 session given only
the README and the releases page) followed the published guides on a clean
machine: Docker running inside a container, nothing else installed. Each run
used a new machine and a new tester with no memory of earlier runs.

Tasks in every run: install 0.2.1 and upgrade it; first-time set-up for
St Ann's; invite a captain; add a player and make their link; back up, copy
off and restore; switch tournaments on; open the in-app help; check the
troubleshooting guide against each stumble.

Limits of the test: there was no web browser, so pages were driven with
`curl` and passkeys were not used (the automated browser suites cover
them); Docker was already installed, so the "install Docker on Ubuntu" step
was read but not run; there was no real domain, so the "Trying it locally"
route was used; BHPLTA had no fixtures published (November), so set-up made
no season.

## Run 1: 1.0.0-rc.1

All tasks completed, with 11 stumbles. One was a failure of the guide's path.

| # | Stumble | Fix |
|---|---|---|
| 1 | A league-only install would not start: Compose demanded tournament secrets | Product: the compose file no longer demands them; a start-up check refuses tournaments without real secrets |
| 2 | Install guide had no "check the version" step | Guide |
| 3 | Trial route skipped `docker compose pull` | Guide |
| 4 | Secrets script said "nothing to do" where the guide promised seven lines | Guide describes both outputs |
| 5 | No season was created (off season); guides assumed one | Product wording on the set-up and Seasons pages; guides; new troubleshooting entry |
| 6 | Club Settings showed "St+Anns" | Not a fault: it is BHPLTA's identifier. Label and hint changed; guides mention it |
| 7 | Quoted output of `docker compose cp` was wrong | Guide quotes real output |
| 8 | Restore drill gave no proof it worked | Guide: before-and-after check; a missing `mkdir` in the drill fixed |
| 9 | Restore cancelled an unused invite with no next step | Guide says how to send a new one |
| 10 | Button quoted as "Save" is "Save settings" | Guide; every quoted label checked |
| 11 | Guide did not say a player with no team can have a link | Guide |

## Run 2: 1.0.0-rc.2

All tasks completed without help. No failures. Four hesitations, all fixed in
the guides: the older-version warning in the trial route came after the
commands; the upgrade and backup guides did not name the trial folder; the
README did not say where in-app help is; the upgrade guide did not say the
change log is one page.

## Run 3: 1.0.0-rc.2, guides from main

All tasks completed without help. No failures. Every command's output matched
the guides. Three minor hesitations, fixed in the guides for 1.0.0:

1. "The first two commands may stop ... then carry on" was vague. Now names
   the command and says to run the block again from the top.
2. The tournaments guide said to press "Set up tournaments" when saving Club
   Settings had already done it. The guide now says so.
3. `/help` needs a sign-in; the README now says to sign in first.
