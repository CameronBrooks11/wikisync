package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
		{"pull --depth below 0", []string{"pull", "--host", "wiki.example.org", "--dir", t.TempDir(), "--category", "C", "--depth", "-1"}, 2},
		{"pull --depth needs --category", []string{"pull", "--host", "wiki.example.org", "--dir", t.TempDir(), "--depth", "1", "Page"}, 2},
		{"pull --depth with --category", []string{"pull", "--host", "wiki.example.org", "--dir", t.TempDir(), "--category", "C", "--depth", "1"}, 1}, // the fake wiki has no pages
		{"pull --watchlist needs credentials", []string{"pull", "--host", "wiki.example.org", "--dir", t.TempDir(), "--watchlist"}, 2},
		{"first pull --watchlist needs --host", []string{"pull", "--dir", t.TempDir(), "--watchlist"}, 2},
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

// cleanTree returns a working tree tracking one page whose file matches its
// baseline, so push has nothing to do for it.
func cleanTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const text = "unchanged\n"
	if err := os.WriteFile(dir+"/Page.wiki", []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteBaseline(dir, 1, text); err != nil {
		t.Fatal(err)
	}
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{
		"Page": {Title: "Page", File: "Page.wiki", Host: "wiki.example.org", Path: "/w/",
			RevID: 1, PageID: 1, SHA256: store.ContentHash(text)},
	}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A title push can't find must not look like a run with nothing to do (#24).
func TestPushReportsUntrackedTitles(t *testing.T) {
	fakeWiki(t)
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"tracked page, nothing changed", []string{"push", "--dir", cleanTree(t), "Page"}, 0},
		{"typo in a tracked tree", []string{"push", "--dir", cleanTree(t), "Pgae"}, 1},
		{"typo alongside a tracked page", []string{"push", "--dir", cleanTree(t), "Page", "Pgae"}, 1},
		{"title in an empty tree", []string{"push", "--dir", t.TempDir(), "Page"}, 1},
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

// capture runs the CLI with stdout and stderr sent to files, and returns the
// exit code and what each received.
func capture(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	outF, err := os.Create(dir + "/stdout")
	if err != nil {
		t.Fatal(err)
	}
	errF, err := os.Create(dir + "/stderr")
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outF, errF
	code = run(args)
	os.Stdout, os.Stderr = origOut, origErr
	_ = outF.Close()
	_ = errF.Close()
	out, _ := os.ReadFile(dir + "/stdout")
	errOut, _ := os.ReadFile(dir + "/stderr")
	return code, string(out), string(errOut)
}

func TestCount(t *testing.T) {
	for n, want := range map[int]string{0: "0 pages", 1: "1 page", 2: "2 pages"} {
		if got := count(n, "page", "pages"); got != want {
			t.Errorf("count(%d) = %q, want %q", n, got, want)
		}
	}
}

// diff and push name each title they were given that isn't tracked, in the
// order given, and still report on the tracked ones.
func TestUntrackedTitlesAreNamed(t *testing.T) {
	fakeWiki(t)
	const pgae = `"Pgae" is not tracked (run 'wikisync pull' first)` + "\n"
	const zed = `"Zed" is not tracked (run 'wikisync pull' first)` + "\n"
	cases := []struct {
		args           []string
		code           int
		stdout, stderr string
	}{
		{[]string{"diff", "--dir", cleanTree(t), "Pgae", "Page", "Zed"}, 0,
			"no local changes\n", "diff: " + pgae + "diff: " + zed},
		{[]string{"push", "--dir", cleanTree(t), "Pgae", "Page", "Zed"}, 1,
			"", "push: " + pgae + "push: " + zed},
		{[]string{"push", "--dir", t.TempDir(), "Pgae"}, 1,
			"", "push: " + pgae},
	}
	for _, tc := range cases {
		code, stdout, stderr := capture(t, tc.args...)
		if code != tc.code || stdout != tc.stdout || stderr != tc.stderr {
			t.Errorf("%v:\n got %d %q %q\nwant %d %q %q", tc.args[:1], code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
		}
	}
}

// Without credentials, pull --watchlist and push --apply name the variables
// to set and what the bot password needs.
func TestLoginNeedsCredentials(t *testing.T) {
	fakeWiki(t)
	for _, v := range []string{"WIKISYNC_USERNAME", "WIKISYNC_PASSWORD",
		"WIKISYNC_USERNAME_EXAMPLE", "WIKISYNC_PASSWORD_EXAMPLE"} {
		t.Setenv(v, "")
	}
	const vars = "WIKISYNC_USERNAME_EXAMPLE and WIKISYNC_PASSWORD_EXAMPLE (or WIKISYNC_USERNAME and WIKISYNC_PASSWORD)"
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"pull", "--dir", trackedTree(t), "--watchlist"},
			"wikisync pull --watchlist: set " + vars + ` to a MediaWiki bot password with the "View your watchlist" grant` + "\n"},
		{[]string{"push", "--dir", trackedTree(t), "--apply", "--summary", "s"},
			"wikisync push --apply: set " + vars + " (a MediaWiki bot password)\n"},
	} {
		if code, _, stderr := capture(t, tc.args...); code != 2 || stderr != tc.want {
			t.Errorf("%v: got %d %q, want 2 %q", tc.args[:1], code, stderr, tc.want)
		}
	}
}

// pull --watchlist logs in before listing, and a failed login stops the pull
// before the watchlist is asked for.
func TestPullWatchlistLogsInFirst(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch {
		case r.Form.Get("meta") == "tokens":
			asked = append(asked, "token")
			_, _ = io.WriteString(w, `{"query":{"tokens":{"logintoken":"t+\\"}}}`)
		case r.Form.Get("action") == "login":
			asked = append(asked, "login")
			_, _ = io.WriteString(w, `{"login":{"result":"Failed","reason":"bad password"}}`)
		default:
			asked = append(asked, r.Form.Get("generator"))
			_, _ = io.WriteString(w, `{"query":{"pages":[]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	orig := newClient
	newClient = func(_, _, ua string) *mw.Client { return mw.NewClientForAPI(srv.URL+"/api.php", ua) }
	t.Cleanup(func() { newClient = orig })
	t.Setenv("WIKISYNC_USERNAME_EXAMPLE", "Me@wikisync")
	t.Setenv("WIKISYNC_PASSWORD_EXAMPLE", "secret")
	code, _, stderr := capture(t, "pull", "--host", "wiki.example.org", "--dir", t.TempDir(), "--watchlist")
	const want = "logging in as Me@wikisync (from WIKISYNC_USERNAME_EXAMPLE)\n" +
		"wikisync pull: login failed: Failed (bad password)\n"
	if code != 1 || stderr != want {
		t.Errorf("got %d %q, want 1 %q", code, stderr, want)
	}
	if fmt.Sprint(asked) != "[token login]" {
		t.Errorf("requests = %v, want a login token, a login, and nothing after the failure", asked)
	}
}

// labServer is a small wiki for pull's output: category Top holds page A, its
// Spanish translation A/es and subcategory Sub, which holds page B; the
// watchlist holds A and a title with no page; Ghost doesn't exist; titles
// starting "User:" are in namespace 2. Logins succeed.
func labServer(t *testing.T) {
	t.Helper()
	page := func(title, lang string, ns int) map[string]any {
		return map[string]any{"title": title, "ns": ns, "pagelanguage": lang}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		var pages []map[string]any
		switch f := r.Form; {
		case f.Get("meta") == "siteinfo":
			_, _ = io.WriteString(w, `{"query":{"general":{"lang":"en"},"namespaces":{}}}`)
			return
		case f.Get("meta") == "tokens":
			_, _ = io.WriteString(w, `{"query":{"tokens":{"logintoken":"t"}}}`)
			return
		case f.Get("action") == "login":
			_, _ = io.WriteString(w, `{"login":{"result":"Success"}}`)
			return
		case f.Get("gcmtitle") == "Category:Top":
			pages = []map[string]any{page("A", "en", 0), page("A/es", "es", 0), page("Category:Sub", "en", 14)}
		case f.Get("gcmtitle") == "Category:Sub":
			pages = []map[string]any{page("B", "en", 0)}
		case f.Get("generator") == "watchlistraw":
			pages = []map[string]any{page("A", "en", 0), {"title": "Gone", "ns": 0, "missing": true}}
		case f.Get("prop") == "revisions":
			for i, title := range strings.Split(f.Get("titles"), "|") {
				if title == "Ghost" {
					pages = append(pages, map[string]any{"title": title, "missing": true})
					continue
				}
				ns := 0
				if strings.HasPrefix(title, "User:") {
					ns = 2
				}
				pages = append(pages, map[string]any{"title": title, "ns": ns, "pageid": i + 1,
					"revisions": []any{map[string]any{"revid": 10, "slots": map[string]any{"main": map[string]any{"content": "text of " + title + "\n"}}}}})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"query": map[string]any{"pages": pages}})
	}))
	t.Cleanup(srv.Close)
	orig := newClient
	newClient = func(_, _, ua string) *mw.Client { return mw.NewClientForAPI(srv.URL+"/api.php", ua) }
	t.Cleanup(func() { newClient = orig })
}

// pull's report: one line per outcome, and exit 1 when anything was skipped
// or nothing was pulled.
func TestPullOutput(t *testing.T) {
	labServer(t)
	t.Setenv("WIKISYNC_USERNAME_EXAMPLE", "Me@wikisync")
	t.Setenv("WIKISYNC_PASSWORD_EXAMPLE", "secret")
	dir := t.TempDir()
	steps := []struct {
		name           string
		args           []string
		edit           string // file in dir to change before the step
		code           int
		stdout, stderr string
	}{
		{name: "category with subcategories",
			args:   []string{"pull", "--host", "wiki.example.org", "--dir", dir, "--category", "Top", "--depth", "1"},
			stdout: `pulled "A" -> A.wiki` + "\n" + `pulled "B" -> B.wiki` + "\n",
			stderr: "walked 2 categories (depth 1)\n" +
				"skipped 1 page not in the wiki's language, such as translations (use --all-languages to include them)\n"},
		{name: "refresh with a local edit", args: []string{"pull", "--dir", dir}, edit: "A.wiki", code: 1,
			stdout: `pulled "B" -> B.wiki` + "\n",
			stderr: "pulling the 2 tracked pages again\n" +
				`skipped "A": local file has unpulled changes (use --force to overwrite)` + "\n"},
		{name: "a page that doesn't exist", args: []string{"pull", "--dir", dir, "Ghost"}, code: 1,
			stderr: `skipped "Ghost": page does not exist` + "\n"},
		{name: "two titles, one filename", args: []string{"pull", "--dir", dir, "User/X", "User:X"}, code: 1,
			stdout: `pulled "User/X" -> User/X.wiki` + "\n",
			stderr: `skipped "User:X": filename "User/X.wiki" is already used by "User/X"` + "\n"},
		{name: "watchlist", args: []string{"pull", "--dir", dir, "--force", "--watchlist"},
			stdout: `pulled "A" -> A.wiki` + "\n",
			stderr: "logging in as Me@wikisync (from WIKISYNC_USERNAME_EXAMPLE)\n" +
				"skipped 1 watched title with no page\n"},
		{name: "empty selection", args: []string{"pull", "--dir", dir, "--category", "Empty"}, code: 1,
			stderr: "no pages matched\n"},
	}
	for _, s := range steps {
		if s.edit != "" {
			if err := os.WriteFile(dir+"/"+s.edit, []byte("local edit\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		code, stdout, stderr := capture(t, s.args...)
		if code != s.code || stdout != s.stdout || stderr != s.stderr {
			t.Errorf("%s:\n got %d %q %q\nwant %d %q %q", s.name, code, stdout, stderr, s.code, s.stdout, s.stderr)
		}
	}
}
