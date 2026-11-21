# Saved BHPLTA pages

Public pages from the BHPLTA website, saved so tests never contact
bhplta.co.uk. The files sit at the same paths as on the real site, so any
static file server rooted at this directory can stand in for it: point the app
at that server with `BHPLTA_BASE_URL=http://<host>:<port>` (no trailing path).

| Path | Source |
|------|--------|
| `bhplta_tables/fixtures/index.html` | The Parks League fixtures page as captured by the Internet Archive on 29 May 2024 (`web.archive.org/web/20240529091319id_/...`). BHPLTA lists only *upcoming* fixtures, so it holds weeks 7 to 18 (divisions 3 and 4 from week 8): 4 divisions, 41 teams, 228 fixtures and 15 byes. |
| `off-season/bhplta_tables/fixtures/index.html` | The same page fetched on 21 Nov 2026, between seasons: "There are no upcoming fixtures". Serve it at `/bhplta_tables/fixtures/` to test the off-season path. |
| `clubs/<slug>/index.html` | The 12 club pages in `services.KnownClubSlugs`, fetched on 21 Nov 2026. The real `/clubs/st-anns/` redirects to `/clubs/st-anns-well-gardens/` (the slug the fixtures page links to), so the St Ann's page is saved under both. |

Club phone numbers and email addresses have been replaced with placeholders
(`01632 960000`, `club@example.invalid`); nothing else was edited.

Importing these pages on an empty database (home club St Ann's) gives:
1 league, 1 season (2024, active), 18 weeks, 4 divisions, 12 clubs, 41 teams
(4 of them St Ann's) and 228 fixtures. `internal/bhplta` and
`internal/setup` unit tests serve this directory with `http.FileServer`.
