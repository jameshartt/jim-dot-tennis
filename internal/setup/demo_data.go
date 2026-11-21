// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

// Demo data: a fictional parks league used to look around the app before
// committing a real club. Every name is invented (surnames are plainly
// made-up place-holders) and nothing here is, or links to, a real person or
// club. Dates are generated relative to the day the demo is loaded.

// demoLeague is the league, season and divisions the demo clubs play in.
const (
	demoLeagueName   = "Demo Parks League"
	demoLeagueRegion = "Demoshire"
)

// demoDivisions are the two fictional divisions (level, name, play day).
var demoDivisions = []struct {
	Name    string
	Level   int
	PlayDay string
}{
	{"Demo Division 1", 1, "Monday"},
	{"Demo Division 2", 2, "Tuesday"},
}

// demoClubs are the three fictional clubs. The first is the home club.
var demoClubs = []struct {
	Name    string
	Address string
	Surname string // stem for the club's invented player surnames
}{
	{"Sample Park Tennis Club", "1 Example Lane, Demotown", "Sample"},
	{"Fictional Vale Tennis Club", "2 Pretend Road, Demotown", "Fict"},
	{"Example Meadow Tennis Club", "3 Placeholder Way, Demotown", "Example"},
}

// Invented first names, handed out four per club in order. Surnames are
// stem + suffix so that none of them is a real family name.
var (
	demoWomen    = []string{"Alice", "Beth", "Carys", "Dina", "Edith", "Freya", "Gwen", "Hana", "Iris", "Jade", "Kira", "Lena"}
	demoMen      = []string{"Adam", "Ben", "Carl", "Dan", "Ed", "Finn", "Gus", "Hugh", "Ian", "Jack", "Kit", "Leo"}
	demoSuffixes = []string{"wood", "field", "ton", "brook", "by", "hurst", "croft", "moor", "ley", "well", "ham", "ford"}
)

// Each club has two teams, A (division 1) and B (division 2), of two women and
// two men. Player i of a club is: A women 0-1, B women 2-3, A men 0-1, B men 2-3.

// demoRounds is the fixture list of one division of three teams (indexes into
// the division's teams; index 0 is the home club's team): home, away, per
// week. Weeks 1-3 are played, week 4 is this week, 5-6 are still to come.
var demoRounds = [][2]int{{1, 0}, {0, 2}, {2, 1}, {0, 1}, {2, 0}, {1, 2}}

// demoWeeks is how many weeks the demo season has; demoCurrentWeek is the one
// containing the day the demo is loaded.
const (
	demoWeeks       = 8
	demoCurrentWeek = 4
)

// A set is home games, away games.
type demoSet [2]int

// demoResult is one rubber's score lines (two or three sets).
type demoResult []demoSet

// demoResults are the scores of the played fixtures, four rubbers each
// (Mens, Womens, 1st Mixed, 2nd Mixed), cycling through for every played
// fixture so the standings and results pages have something varied to show.
var demoResults = [][4]demoResult{
	{{{6, 3}, {6, 4}}, {{6, 2}, {6, 1}}, {{6, 4}, {3, 6}, {6, 3}}, {{7, 5}, {6, 4}}},
	{{{3, 6}, {4, 6}}, {{6, 4}, {6, 3}}, {{2, 6}, {6, 4}, {3, 6}}, {{6, 3}, {7, 6}}},
	{{{6, 4}, {6, 2}}, {{4, 6}, {3, 6}}, {{6, 1}, {6, 0}}, {{3, 6}, {6, 4}, {6, 4}}},
	{{{6, 4}, {4, 6}, {3, 6}}, {{6, 3}, {6, 3}}, {{4, 6}, {2, 6}}, {{6, 0}, {6, 2}}},
	{{{6, 3}, {6, 3}}, {{6, 3}, {2, 6}, {6, 4}}, {{3, 6}, {4, 6}}, {{6, 2}, {6, 4}}},
	{{{2, 6}, {3, 6}}, {{6, 4}, {6, 4}}, {{6, 3}, {6, 4}}, {{4, 6}, {6, 3}, {2, 6}}},
}

// demoAvailability cycles over the home club's players for the upcoming weeks.
var demoAvailability = []string{"Available", "Available", "IfNeeded", "Unavailable", "Available", "Unknown", "Available", "Available"}
