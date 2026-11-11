package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

// silence redirects stdout/stderr to /dev/null for the duration of a test, so
// the CLI's normal output doesn't clutter `go test`.
func silence(t *testing.T) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devnull, devnull
	t.Cleanup(func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = devnull.Close()
	})
}

// wikiDialed is the host and path a command asked for a client to.
type wikiDialed struct{ host, path string }

// fakeWiki points every client the CLI builds at a local server that returns no
// pages, so no test reaches a real wiki, and records which wiki was asked for.
func fakeWiki(t *testing.T) *wikiDialed {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"query":{"pages":[]}}`)
	}))
	t.Cleanup(srv.Close)
	dialed := &wikiDialed{}
	orig := newClient
	newClient = func(host, path, ua string) *mw.Client {
		dialed.host, dialed.path = host, path
		return mw.NewClientForAPI(srv.URL+"/api.php", ua)
	}
	t.Cleanup(func() { newClient = orig })
	return dialed
}

// trackedTree returns a working tree whose manifest tracks one page from
// wiki.example.org.
func trackedTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{
		"Page": {Title: "Page", File: "Page.wiki", Host: "wiki.example.org", Path: "/w/"},
	}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRun(t *testing.T) {
	fakeWiki(t)
	// Keep the result independent of credentials in the developer's shell.
	for _, v := range []string{"WIKISYNC_USERNAME", "WIKISYNC_PASSWORD",
		"WIKISYNC_USERNAME_EXAMPLE", "WIKISYNC_PASSWORD_EXAMPLE"} {
		t.Setenv(v, "")
	}
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no args prints usage", []string{}, 0},
		{"version", []string{"--version"}, 0},
		{"unknown command", []string{"bogus"}, 2},
		{"pull with nothing named or tracked", []string{"pull", "--dir", t.TempDir()}, 2},
		{"first pull needs --host", []string{"pull", "--dir", t.TempDir(), "Page"}, 2},
		{"pull refuses another wiki", []string{"pull", "--host", "other.example.org", "--dir", trackedTree(t), "Page"}, 2},
		{"status on empty dir", []string{"status", "--dir", t.TempDir()}, 0},
		{"diff on empty dir", []string{"diff", "--dir", t.TempDir()}, 0},
		{"push preview, nothing to push", []string{"push", "--dir", t.TempDir()}, 0},
		{"push apply needs summary", []string{"push", "--apply", "--dir", t.TempDir()}, 2},
		{"push apply needs credentials", []string{"push", "--apply", "--summary", "s", "--dir", trackedTree(t)}, 2},
		{"push refuses another wiki", []string{"push", "--host", "other.example.org", "--dir", trackedTree(t)}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silence(t)
			if got := run(tc.args); got != tc.want {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

// Commands in a working tree talk to the wiki recorded in it, not a default.
func TestCommandsUseTheTreesWiki(t *testing.T) {
	dialed := fakeWiki(t)
	for _, cmd := range []string{"pull", "push"} {
		*dialed = wikiDialed{}
		silence(t)
		run([]string{cmd, "--dir", trackedTree(t)})
		if dialed.host != "wiki.example.org" || dialed.path != "/w/" {
			t.Errorf("%s connected to %q %q, want the tree's wiki.example.org /w/", cmd, dialed.host, dialed.path)
		}
	}
}
