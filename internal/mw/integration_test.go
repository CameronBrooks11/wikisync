//go:build integration

// Watchlist listing against a real MediaWiki: the throwaway wiki that
// `just test-integration` starts (scripts/test-wiki.sh). It adds pages to the
// account's watchlist, so it runs only against WIKISYNC_TEST_API_URL, never a
// real wiki's host. Configured by:
//
//	WIKISYNC_TEST_API_URL   the test wiki's api.php
//	WIKISYNC_TEST_PAGE      an existing page to watch
//	WIKISYNC_USERNAME, WIKISYNC_PASSWORD
//	                        a bot password with the viewmywatchlist and
//	                        editmywatchlist grants
//	WIKISYNC_TEST_NOGRANT_USERNAME, WIKISYNC_TEST_NOGRANT_PASSWORD
//	                        a bot password without them
//	WIKISYNC_REQUIRE_INTEGRATION  when set, missing config fails instead of skipping
package mw

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveWatchlist(t *testing.T) {
	const ua = "wikisync-integration-test (+https://github.com/CameronBrooks11/wikisync)"
	apiURL, page := os.Getenv("WIKISYNC_TEST_API_URL"), os.Getenv("WIKISYNC_TEST_PAGE")
	user, pass := os.Getenv("WIKISYNC_USERNAME"), os.Getenv("WIKISYNC_PASSWORD")
	noUser, noPass := os.Getenv("WIKISYNC_TEST_NOGRANT_USERNAME"), os.Getenv("WIKISYNC_TEST_NOGRANT_PASSWORD")
	if apiURL == "" || page == "" || user == "" || pass == "" || noUser == "" || noPass == "" {
		msg := "set WIKISYNC_TEST_API_URL, WIKISYNC_TEST_PAGE, WIKISYNC_USERNAME/PASSWORD and WIKISYNC_TEST_NOGRANT_USERNAME/PASSWORD to run"
		if os.Getenv("WIKISYNC_REQUIRE_INTEGRATION") != "" {
			t.Fatal(msg)
		}
		t.Skip(msg)
	}

	if _, _, err := NewClientForAPI(apiURL, ua).WatchlistPages(); err == nil || !strings.Contains(err.Error(), "notloggedin") {
		t.Errorf("anonymous: err = %v, want notloggedin", err)
	}

	c := NewClientForAPI(apiURL, ua)
	if err := c.Login(user, pass); err != nil {
		t.Fatal(err)
	}
	absent := fmt.Sprintf("Wikisync no such page %d", time.Now().UnixNano())
	watch(t, c, page+"|"+absent)

	pages, missing, err := c.WatchlistPages()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(page, "_", " ")
	found := false
	for _, p := range pages {
		switch {
		case p.Title == want:
			found = true
		case p.NS%2 == 1 || p.Title == absent:
			t.Errorf("listed %q: talk pages and missing pages must be left out", p.Title)
		}
	}
	if !found || missing < 1 {
		t.Errorf("pages = %v, missing = %d; want %q listed and %q counted as missing", pages, missing, want, absent)
	}

	nogrant := NewClientForAPI(apiURL, ua)
	if err := nogrant.Login(noUser, noPass); err != nil {
		t.Fatal(err)
	}
	if _, _, err := nogrant.WatchlistPages(); err == nil || !strings.Contains(err.Error(), `"View your watchlist" grant`) {
		t.Errorf("without the grant: err = %v, want the grant named", err)
	}
}

// watch adds titles ("A|B") to the logged-in user's watchlist.
func watch(t *testing.T, c *Client, titles string) {
	t.Helper()
	tok, err := c.token("watch")
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Watch []struct {
			Title   string `json:"title"`
			Watched bool   `json:"watched"`
		} `json:"watch"`
	}
	if err := c.call(http.MethodPost, url.Values{
		"action": {"watch"}, "titles": {titles}, "token": {tok},
	}, &out); err != nil {
		t.Fatalf("watch %s: %v", titles, err)
	}
	if n := strings.Count(titles, "|") + 1; len(out.Watch) != n {
		t.Fatalf("watch %s: %+v", titles, out.Watch)
	}
}
