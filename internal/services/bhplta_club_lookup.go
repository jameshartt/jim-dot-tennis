// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jim-dot-tennis/internal/version"

	"github.com/PuerkitoBio/goquery"
)

// BHPLTAClubIdentity is a club's identity on BHPLTA's club scores page.
type BHPLTAClubIdentity struct {
	Name string // as sent in match-card requests (data-clubname)
	ID   int    // data-clubid
}

// BHPLTAClubLookup finds a club's BHPLTA name and id from its club code.
//
// BHPLTA does not publish club ids. Its club scores page, after the club
// password is entered, renders a heading carrying data-clubid and
// data-clubname, which the page's own script then sends with every
// match-card request. Lookup makes the same two requests the page does: load
// the page for a nonce, then ask for the club's scores with the password.
//
// This follows the page's script (bhplta-club-scores JS) rather than a
// documented API, so it may stop working if BHPLTA changes the page.
type BHPLTAClubLookup struct {
	client  *http.Client
	pageURL string
	ajaxURL string
}

// NewBHPLTAClubLookup returns a lookup against the live BHPLTA site.
func NewBHPLTAClubLookup() *BHPLTAClubLookup {
	return &BHPLTAClubLookup{
		client:  &http.Client{Timeout: 30 * time.Second},
		pageURL: "https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/",
		ajaxURL: "https://www.bhplta.co.uk/wp-admin/admin-ajax.php",
	}
}

// Lookup returns the club identity for clubCode.
func (l *BHPLTAClubLookup) Lookup(ctx context.Context, clubCode string) (BHPLTAClubIdentity, error) {
	if strings.TrimSpace(clubCode) == "" {
		return BHPLTAClubIdentity{}, fmt.Errorf("set the BHPLTA club code first")
	}
	extractor := &NonceExtractor{httpClient: l.client, baseURL: l.pageURL}
	nonce, err := extractor.ExtractNonceWithClubCode(clubCode)
	if err != nil {
		return BHPLTAClubIdentity{}, fmt.Errorf("could not load the BHPLTA club scores page: %w", err)
	}

	form := url.Values{}
	form.Set("nonce", nonce.Nonce)
	form.Set("action", "bhplta_club_scores_get_scores")
	form.Set("passcode", clubCode)
	form.Set("query_type", "passcode")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.ajaxURL, strings.NewReader(form.Encode()))
	if err != nil {
		return BHPLTAClubIdentity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", l.pageURL)
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Cookie", "clubcode="+clubCode)

	resp, err := l.client.Do(req)
	if err != nil {
		return BHPLTAClubIdentity{}, fmt.Errorf("BHPLTA request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BHPLTAClubIdentity{}, &HTTPStatusError{URL: l.ajaxURL, Status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return BHPLTAClubIdentity{}, fmt.Errorf("read BHPLTA response: %w", err)
	}
	return ParseBHPLTAClubScoresResponse(body)
}

// ParseBHPLTAClubScoresResponse extracts the club identity from the club
// scores response: JSON {"status": ..., "output": "<html>"} or bare HTML.
func ParseBHPLTAClubScoresResponse(body []byte) (BHPLTAClubIdentity, error) {
	html := string(body)
	var payload struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		switch payload.Status {
		case "no_passcode", "club_name_error", "errors":
			msg := strings.TrimSpace(payload.Output)
			if doc, err := goquery.NewDocumentFromReader(strings.NewReader(msg)); err == nil {
				msg = strings.TrimSpace(doc.Text())
			}
			if msg == "" {
				msg = payload.Status
			}
			return BHPLTAClubIdentity{}, fmt.Errorf("BHPLTA did not accept the club code: %s", msg)
		}
		html = payload.Output
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return BHPLTAClubIdentity{}, fmt.Errorf("could not read the BHPLTA response: %w", err)
	}
	var id BHPLTAClubIdentity
	doc.Find("[data-clubid]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		n, err := strconv.Atoi(strings.TrimSpace(s.AttrOr("data-clubid", "")))
		if err != nil || n <= 0 {
			return true
		}
		id = BHPLTAClubIdentity{ID: n, Name: strings.TrimSpace(s.AttrOr("data-clubname", ""))}
		return false
	})
	if id.ID == 0 || id.Name == "" {
		return BHPLTAClubIdentity{}, fmt.Errorf("the BHPLTA response did not include the club's name and id (the page may have changed)")
	}
	return id, nil
}
