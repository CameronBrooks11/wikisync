//go:build integration

// Live push round-trip against a real wiki. Opt-in: build tag `integration`
// plus env vars, so the default `go test ./...` and CI never compile or run it.
//
//	WIKISYNC_TEST_PAGE      page title to edit (use a designated test/sandbox page)
//	WIKISYNC_USERNAME       MediaWiki bot-password username
//	WIKISYNC_PASSWORD       MediaWiki bot password
//	WIKISYNC_TEST_HOST      optional, default www.appropedia.org
//	WIKISYNC_TEST_PATH      optional, default /w/
//
// Run with: just test-integration   (writes to, then restores, the test page).
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CameronBrooks11/wikisync/internal/mw"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestLivePushRoundTrip(t *testing.T) {
	page := os.Getenv("WIKISYNC_TEST_PAGE")
	user := os.Getenv("WIKISYNC_USERNAME")
	pass := os.Getenv("WIKISYNC_PASSWORD")
	if page == "" || user == "" || pass == "" {
		t.Skip("set WIKISYNC_TEST_PAGE, WIKISYNC_USERNAME and WIKISYNC_PASSWORD to run")
	}
	host := envOr("WIKISYNC_TEST_HOST", "www.appropedia.org")
	path := envOr("WIKISYNC_TEST_PATH", "/w/")

	client := mw.NewClient(host, path, "wikisync-integration-test (+https://github.com/CameronBrooks11/wikisync)")
	dir := t.TempDir()

	// 1. Pull the designated page.
	rep, err := Pull([]string{page}, client.FetchPage, dir, host, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 1 {
		t.Fatalf("expected to pull one page, got %+v", rep)
	}
	file := filepath.Join(dir, rep.Written[0].File)
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Login(user, pass); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Always try to restore the page to its original content.
	t.Cleanup(func() {
		if err := os.WriteFile(file, original, 0o644); err != nil {
			t.Logf("restore: writing original back locally failed: %v", err)
			return
		}
		if _, err := Push(dir, nil, client, true, "wikisync integration test: restore"); err != nil {
			t.Logf("restore push failed; the test page may need manual cleanup: %v", err)
		}
	})

	// 2. Make a harmless, unique local edit.
	marker := fmt.Sprintf("<!-- wikisync integration test %d -->", time.Now().UnixNano())
	edited := append(append([]byte{}, original...), []byte("\n"+marker+"\n")...)
	if err := os.WriteFile(file, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Push it for real.
	results, err := Push(dir, nil, client, true, "wikisync integration test: edit")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != Pushed {
		t.Fatalf("push results = %+v", results)
	}

	// 4. Verify the remote page now carries the marker.
	after, err := client.FetchPage(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after.Text, marker) {
		t.Errorf("remote page does not contain the pushed marker")
	}
	if after.RevID == 0 {
		t.Errorf("expected a non-zero revision id after the push")
	}
}
