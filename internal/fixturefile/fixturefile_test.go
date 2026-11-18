// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package fixturefile

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The sample is written by the JavaScript generator
// (tests/fixture-generator/make-sample.mjs), so these tests also pin the two
// implementations of the format to each other.
func sample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/sample-2027.jimtennis.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	return data
}

// edit parses the sample into a generic map, applies fn and re-encodes it.
func edit(t *testing.T, fn func(file map[string]interface{})) []byte {
	t.Helper()
	var file map[string]interface{}
	if err := json.Unmarshal(sample(t), &file); err != nil {
		t.Fatal(err)
	}
	fn(file)
	out, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func firstDivision(file map[string]interface{}) map[string]interface{} {
	return file["divisions"].([]interface{})[0].(map[string]interface{})
}

func TestParseGeneratedFile(t *testing.T) {
	f, err := Parse(bytes.NewReader(sample(t)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Season.Year != 2027 || f.Season.Weeks != 18 {
		t.Errorf("season: %+v", f.Season)
	}
	if f.Status != "accepted" {
		t.Errorf("status: got %q, want accepted", f.Status)
	}
	if got := len(f.Divisions); got != 4 {
		t.Fatalf("divisions: got %d, want 4", got)
	}
	if f.FixtureCount() != 402 {
		t.Errorf("fixtures: got %d, want 402", f.FixtureCount())
	}
	if d := f.Divisions[0]; d.PlayDay != "Thursday" || d.Level != 1 {
		t.Errorf("division 1 config: %+v", d)
	}
	if got := f.Divisions[0].Fixtures[0].Time().Format(dateLayout); got != "2027-04-15" {
		t.Errorf("first Division 1 fixture: got %s, want 2027-04-15", got)
	}
}

func TestChecksumMatchesGenerator(t *testing.T) {
	f, err := Parse(bytes.NewReader(sample(t)))
	if err != nil {
		t.Fatal(err)
	}
	if f.Checksum == "" {
		t.Fatal("sample has no checksum")
	}
	if got := Checksum(f.Divisions); got != f.Checksum {
		t.Errorf("Go checksum %s differs from the generator's %s", got, f.Checksum)
	}
	if !f.Intact() {
		t.Error("an untouched file should be intact")
	}

	// Moving one fixture to another date leaves a valid file that is no longer as generated.
	moved := edit(t, func(file map[string]interface{}) {
		fixtures := firstDivision(file)["fixtures"].([]interface{})
		fixtures[0].(map[string]interface{})["date"] = "2027-09-01"
	})
	g, err := Parse(bytes.NewReader(moved))
	if err != nil {
		t.Fatalf("edited file should still parse: %v", err)
	}
	if g.Intact() {
		t.Error("an edited file should not be intact")
	}
}

func TestParseRejectsUnusableFiles(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"not json", []byte("Week,Date,Home\n1,April 17,Dyke"), "not a jim.tennis fixture file"},
		{"other json", []byte(`{"format":"something/else","version":1}`), "not a jim.tennis fixture file"},
		{"newer version", edit(t, func(f map[string]interface{}) { f["version"] = 99 }), "version 99"},
		{"no divisions", edit(t, func(f map[string]interface{}) { f["divisions"] = []interface{}{} }), "no divisions"},
		{"missing fixture", edit(t, func(f map[string]interface{}) {
			d := firstDivision(f)
			d["fixtures"] = d["fixtures"].([]interface{})[1:]
		}), "exactly once"},
		{"unknown team", edit(t, func(f map[string]interface{}) {
			firstDivision(f)["fixtures"].([]interface{})[0].(map[string]interface{})["home"] = "Nobody"
		}), "not in the division"},
		{"bad week", edit(t, func(f map[string]interface{}) {
			firstDivision(f)["fixtures"].([]interface{})[0].(map[string]interface{})["week"] = 19
		}), "week 19"},
		{"bad date", edit(t, func(f map[string]interface{}) {
			firstDivision(f)["fixtures"].([]interface{})[0].(map[string]interface{})["date"] = "17 April"
		}), "no valid date"},
		{"team in two divisions", edit(t, func(f map[string]interface{}) {
			divisions := f["divisions"].([]interface{})
			name := firstDivision(f)["teams"].([]interface{})[0].(map[string]interface{})["name"]
			second := divisions[1].(map[string]interface{})
			second["teams"] = append(second["teams"].([]interface{}), map[string]interface{}{"name": name})
		}), "listed in both"},
		{"too large", bytes.Repeat([]byte(" "), MaxSize+10), "too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(bytes.NewReader(tc.data))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestNamesAreNormalised(t *testing.T) {
	curly := bytes.ReplaceAll(sample(t), []byte("St Ann's"), []byte("St Ann’s"))
	f, err := Parse(bytes.NewReader(curly))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, d := range f.Divisions {
		for _, team := range d.Teams {
			if strings.ContainsRune(team.Name, '’') {
				t.Fatalf("curly apostrophe survived in %q", team.Name)
			}
		}
	}
	if !f.Intact() {
		t.Error("apostrophe style should not affect the checksum")
	}

	for team, club := range map[string]string{"Dyke A": "Dyke", "King Alfred B": "King Alfred", "Park Avenue": "Park Avenue", "St Ann’s": "St Ann's"} {
		if got := ClubOf(team); got != club {
			t.Errorf("ClubOf(%q) = %q, want %q", team, got, club)
		}
	}
}
