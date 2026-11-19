# Fixture generator

`/fixture-generator` turns one season's final league tables into the next
season's fixtures. It is public (no login) and writes nothing to the database:
the committee can open it on any jim.tennis site, work through the four steps,
and download a fixture file that every club loads into its own site.

## The four steps

1. **Final tables.** Read live from the BHPLTA league tables page
   (`LEAGUE_TABLES_URL` overrides the address). If the league website cannot be
   reached, the bundled copy of the 2026 tables is used and the page says so.
2. **Promotion and relegation.** Start with *Who is entering next season?*:
   remove a team a club is dropping, or add a new entry to any division.
   Promotion and relegation are worked out again at once, since one team
   leaving can unblock another (with Saltdean D gone, Saltdean B can be
   relegated). The rules are then applied automatically and every non-routine
   decision is listed as a ruling with its reason. The committee can overrule:
   tap a team to move it or mark a promotion as declined. Fixtures cannot be
   generated while a division breaks the club cap.
3. **Fixtures.** An 18-week double round robin per division. Every press of
   *Generate fixtures* or *Draw again* is a new, numbered draw; *Accept* locks
   it. Tap a date to move a round (that keeps the current draw's seed).
4. **Share.** Download the fixture file or a CSV, print fixture cards (one A4
   page per division; the type shrinks so that even a 14-team division fits,
   and anything larger breaks between cards), or print every division side by
   side on one large made-to-measure sheet.

## Rules applied

From the BHPLTA rules (league regulations 1, 2 and 10):

- Top two promoted, bottom two relegated, between all divisions.
- Ties on points are split on sets won minus sets lost. Teams still level are
  flagged: the rules call for a deciding match.
- **Rule 2(b):** no more than two teams from one club in a division. A blocked
  promotion is offered to the next-placed team (how far down it may pass is a
  setting; "next team only" is the rule as written). If nobody takes it, the
  team next to bottom in the division above is not relegated.
- Division 1 plays Thursdays, Division 2 Wednesdays, Divisions 3 and 4
  Tuesdays.

One case the rules do not spell out: a team that cannot be **relegated**
because its club already has two teams in the division below. The generator
keeps that team up and promotes one fewer team in exchange, so division sizes
do not change. The committee can overrule it like any other ruling.

## How fixtures are drawn

- Each division is a single round robin in weeks 1–9, repeated with venues
  swapped in weeks 10–18 (week *w* mirrors week *w* + 9).
- A division with more than ten teams has more rounds than weeks. The extra
  rounds go in the opening and closing weeks of each half first (weeks 1 and 9,
  so 10 and 18). Following past practice the extra round of week 1 is played a
  week early, that of week 18 a week late, and others on the Saturday.
- The search then arranges the draw so that, in order of importance:
  1. no club hosts more than half the teams it has out on one day (clubs share
     courts across divisions that play the same night);
  2. each team's first half splits evenly between home and away;
  3. runs of three or more home (or away) matches are rare;
  4. same-club derbies fall in the first five weeks of each half.
- The draw is reproducible: the file records the seed.

Strict home, away, home, away for every team is not possible. Two teams with
the same alternating pattern are at home on the same weeks and away on the
same weeks, so they could never meet; there are only two alternating patterns,
so at most two teams in a division can alternate all season. Everyone else
must play two in a row somewhere. The search keeps those runs short. Pushing
harder on runs costs court sharing: on the 2026 tables, weighting runs twelve
times more heavily cut the teams with a run of three from about nine to three,
but put a club over its share of home matches on three or four nights.

## The fixture file

`<league>-<year>-fixtures.jimtennis.json`, format `jim.tennis/fixtures`
version 1: the league, the season, and for each division its play day, teams
(with club) and fixtures (`week`, `date`, `home`, `away`, and `extra` for the
second round of a double-header week). `status` is `draft` or `accepted`.
`checksum` lets a reader tell whether the fixtures have been edited since they
were generated; it guards against accidents, not forgery.

Both readers (`static/fixture-generator/engine.js` and
`internal/fixturefile`) refuse a file unless every pair of teams in a division
meets exactly once at each venue and nobody plays twice on one day.

## Loading a file into a club's site

Admin → Seasons → create the season (18 weeks) → Setup → Import → choose the
file. The season must be the file's year and have no divisions yet. Divisions,
teams and fixtures are created; clubs the site has not seen are created by
name; players and captains are copied from last season's teams of the same
name when that box is ticked.

## Code

| Where | What |
| --- | --- |
| `static/fixture-generator/engine.js` | Rules, scheduler, file format. No DOM. |
| `static/fixture-generator/app.js`, `app.css` | The interface. |
| `internal/fixturegen` | Page route and the league-tables feed. |
| `internal/fixturefile` | Go reader and validator for the file. |
| `internal/admin` (`ImportFixtureFile`) | Loading a file into a season. |
| `tests/fixture-generator` | Engine tests: `make test-js`. |
| `tests/e2e/workflows/fixture-generator.spec.ts` | Browser tests. |

`node tests/fixture-generator/make-sample.mjs` regenerates the sample file the
Go and JavaScript tests share; run it after a deliberate change to the engine
or the format.
