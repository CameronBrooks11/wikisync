//go:build integration

// Push round-trip against a real MediaWiki. Build tag `integration`, so the
// default `go test ./...` never compiles it. Configured by env vars:
//
//	WIKISYNC_TEST_PAGE      page title to edit (use a designated test page)
//	WIKISYNC_USERNAME[_<WIKI>], WIKISYNC_PASSWORD[_<WIKI>]
//	                        bot-password credentials, looked up as push does
//	                        (<WIKI> from the host, e.g. APPROPEDIA)
//	WIKISYNC_TEST_API_URL   optional full api.php URL (e.g. a local test wiki)
//	WIKISYNC_TEST_HOST      optional, default www.appropedia.org (https)
//	WIKISYNC_TEST_PATH      optional, default /w/
//	WIKISYNC_REQUIRE_INTEGRATION  when set, missing config fails instead of skipping
//
// Run with:
//
//	just test-integration   throwaway MediaWiki in Docker (what CI runs)
//	just test-live          a real wiki you have a test page and bot password on
//
// Each run adds about three revisions to the test page (an edit, an
// out-of-band edit for the conflict check, and a restore) and leaves its text
// byte-identical to how it started.
package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CameronBrooks11/wikisync/internal/creds"
	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestLivePushRoundTrip(t *testing.T) {
	stamp := time.Now().UnixNano()
	const ua = "wikisync-integration-test (+https://github.com/CameronBrooks11/wikisync)"

	host := envOr("WIKISYNC_TEST_HOST", "www.appropedia.org")
	path := envOr("WIKISYNC_TEST_PATH", "/w/")
	client := mw.NewClient(host, path, ua)
	if apiURL := os.Getenv("WIKISYNC_TEST_API_URL"); apiURL != "" {
		u, err := url.Parse(apiURL)
		if err != nil {
			t.Fatalf("WIKISYNC_TEST_API_URL: %v", err)
		}
		host, path = u.Host, strings.TrimSuffix(u.Path, "api.php")
		client = mw.NewClientForAPI(apiURL, ua)
	}

	// Credentials are found the same way push finds them.
	page := os.Getenv("WIKISYNC_TEST_PAGE")
	c, haveCreds := creds.Lookup(host, os.Getenv)
	if page == "" || !haveCreds {
		msg := "set WIKISYNC_TEST_PAGE and " + creds.Hint(host) + " to run"
		if os.Getenv("WIKISYNC_REQUIRE_INTEGRATION") != "" {
			t.Fatal(msg) // a required run must not pass by skipping
		}
		t.Skip(msg)
	}
	user, pass := c.Username, c.Password
	dir := t.TempDir()

	original, err := client.FetchPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if !original.Exists {
		t.Fatalf("test page %q does not exist; create it first", page)
	}
	if err := client.Login(user, pass); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Restore the fixture with a direct API edit (no baseline involved), so it
	// works even after the conflict step has made our baseline stale.
	t.Cleanup(func() {
		cur, err := client.FetchPage(page)
		if err != nil {
			t.Errorf("restore: fetch failed: %v", err)
			return
		}
		if cur.Text != original.Text {
			if _, err := client.Edit(page, original.Text, "wikisync integration test: restore", 0); err != nil {
				t.Errorf("restore edit failed; the test page needs manual cleanup: %v", err)
				return
			}
		}
		after, err := client.FetchPage(page)
		if err != nil {
			t.Errorf("restore: verify fetch failed: %v", err)
			return
		}
		if after.Text != original.Text {
			t.Errorf("restore: page text differs from the original fixture")
		}
	})

	// --- 1. pull -----------------------------------------------------------
	rep, err := Pull([]string{page}, client.FetchPage, dir, host, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 1 {
		t.Fatalf("pull: expected one page, got %+v", rep)
	}
	title := rep.Written[0].Title
	file := filepath.Join(dir, rep.Written[0].File)

	// assertSynced checks that the manifest, baseline and working file all
	// match what the wiki actually stores right now.
	assertSynced := func(step string) mw.FetchedPage {
		t.Helper()
		live, err := client.FetchPage(page)
		if err != nil {
			t.Fatalf("%s: fetch: %v", step, err)
		}
		m, err := store.LoadManifest(dir)
		if err != nil {
			t.Fatalf("%s: manifest: %v", step, err)
		}
		entry := m.Entries[title]
		if entry.RevID != live.RevID {
			t.Errorf("%s: manifest revid = %d, wiki revid = %d", step, entry.RevID, live.RevID)
		}
		baseline, ok, err := store.ReadBaseline(dir, entry.PageID)
		if err != nil || !ok {
			t.Errorf("%s: baseline missing (ok=%v err=%v)", step, ok, err)
		} else if baseline != live.Text {
			t.Errorf("%s: baseline text differs from the wiki's stored text (%d vs %d bytes)", step, len(baseline), len(live.Text))
		}
		if local := mustRead(t, file); local != live.Text {
			t.Errorf("%s: local file differs from the wiki's stored text (%d vs %d bytes)", step, len(local), len(live.Text))
		}
		items, err := Status(dir)
		if err != nil {
			t.Fatalf("%s: status: %v", step, err)
		}
		if len(items) != 1 || items[0].State != Unchanged {
			t.Errorf("%s: status = %+v, want unchanged", step, items)
		}
		return live
	}

	assertSynced("after pull")

	// --- 2. edit + push ------------------------------------------------------
	// The trailing newline is deliberate: MediaWiki trims it on save, so this
	// checks that the baseline records what the wiki stored, not what we sent.
	marker := fmt.Sprintf("<!-- wikisync integration test %d -->", stamp)
	mustWrite(t, file, original.Text+"\n"+marker+"\n")
	results, err := Push(dir, nil, client, true, "wikisync integration test: edit")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != Pushed {
		t.Fatalf("push: results = %+v", results)
	}
	live := assertSynced("after push")
	if !strings.Contains(live.Text, marker) {
		t.Errorf("after push: wiki text does not contain the marker")
	}

	// --- 3. pushing again is a no-op ----------------------------------------
	results, err = Push(dir, nil, client, true, "wikisync integration test: should not happen")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != SkipUnchanged {
		t.Errorf("second push: results = %+v, want unchanged", results)
	}
	if again := assertSynced("after second push"); again.RevID != live.RevID {
		t.Errorf("second push created revision %d", again.RevID)
	}

	// --- 4. whitespace-only change: the wiki makes no new revision ----------
	mustWrite(t, file, mustRead(t, file)+"\n")
	results, err = Push(dir, nil, client, true, "wikisync integration test: trailing newline")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action == PushError || results[0].Action == Conflict {
		t.Errorf("whitespace-only push: results = %+v", results)
	}
	if after := assertSynced("after whitespace-only push"); after.RevID != live.RevID {
		t.Errorf("whitespace-only push changed the revision %d -> %d", live.RevID, after.RevID)
	}

	// --- 5. conflict: the wiki moves under us ---------------------------------
	outOfBand := live.Text + fmt.Sprintf("\n<!-- out-of-band edit %d -->", stamp)
	if _, err := client.Edit(page, outOfBand, "wikisync integration test: out-of-band edit", 0); err != nil {
		t.Fatalf("out-of-band edit: %v", err)
	}
	mustWrite(t, file, mustRead(t, file)+"\n<!-- local edit that must not land -->")
	for _, apply := range []bool{false, true} {
		results, err = Push(dir, nil, client, apply, "wikisync integration test: must be refused")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].Action != Conflict {
			t.Errorf("conflict (apply=%v): results = %+v, want conflict", apply, results)
		}
	}
	afterConflict, err := client.FetchPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if afterConflict.Text != outOfBand {
		t.Errorf("conflict: the refused push changed the wiki text")
	}
}
