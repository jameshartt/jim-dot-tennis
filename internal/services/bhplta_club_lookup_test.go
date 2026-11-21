// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The markup mirrors what BHPLTA's club scores script reads:
// $('h1').data('clubid') and $('h1').data('clubname').
const clubScoresOutput = `<div class="bhplta_title_wrapper"><h1 data-clubid="42" data-clubname="Alpha+Park" data-passcode="x">Alpha Park</h1></div><select id="bhplta-clubs"><option value="42">Alpha Park</option></select>`

func TestParseBHPLTAClubScoresResponse(t *testing.T) {
	body := []byte(`{"status":"success","output":` + jsonString(clubScoresOutput) + `}`)
	id, err := ParseBHPLTAClubScoresResponse(body)
	if err != nil || id.ID != 42 || id.Name != "Alpha+Park" {
		t.Fatalf("JSON response: %+v, %v", id, err)
	}
	id, err = ParseBHPLTAClubScoresResponse([]byte(clubScoresOutput))
	if err != nil || id.ID != 42 {
		t.Fatalf("bare HTML: %+v, %v", id, err)
	}
	if _, err := ParseBHPLTAClubScoresResponse([]byte(`{"status":"club_name_error","output":"<p>Incorrect password</p>"}`)); err == nil || !strings.Contains(err.Error(), "Incorrect password") {
		t.Fatalf("error status: %v", err)
	}
	if _, err := ParseBHPLTAClubScoresResponse([]byte(`{"status":"success","output":"<h1>No data attributes</h1>"}`)); err == nil {
		t.Fatalf("missing attributes accepted")
	}
}

func TestBHPLTAClubLookupMakesThePagesRequests(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			if c, _ := r.Cookie("clubcode"); c == nil || c.Value != "ALPHA01" {
				t.Errorf("page request without club code cookie")
			}
			io.WriteString(w, `<html><script>var my_ajax_object2 = {"ajax_url":"x","nonce":"abc123"};</script></html>`)
		case "/ajax":
			_ = r.ParseForm()
			gotForm = r.PostForm
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"status":"success","output":`+jsonString(clubScoresOutput)+`}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	l := &BHPLTAClubLookup{client: srv.Client(), pageURL: srv.URL + "/page", ajaxURL: srv.URL + "/ajax"}
	id, err := l.Lookup(context.Background(), "ALPHA01")
	if err != nil || id.ID != 42 || id.Name != "Alpha+Park" {
		t.Fatalf("Lookup = %+v, %v", id, err)
	}
	if gotForm.Get("nonce") != "abc123" || gotForm.Get("action") != "bhplta_club_scores_get_scores" || gotForm.Get("passcode") != "ALPHA01" || gotForm.Get("query_type") != "passcode" {
		t.Errorf("ajax form = %v", gotForm)
	}
	if _, err := l.Lookup(context.Background(), " "); err == nil {
		t.Errorf("empty code accepted")
	}
}

func jsonString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
