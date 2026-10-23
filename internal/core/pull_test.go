package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

// --- shared test helpers (package core) ---

func fakeFetch(pages map[string]mw.FetchedPage) FetchFunc {
	return func(title string) (mw.FetchedPage, error) { return pages[title], nil }
}

func page(title, text string, pageID, revID int64) mw.FetchedPage {
	return mw.FetchedPage{Title: title, Text: text, RevID: revID, PageID: pageID, Exists: true}
}

func mustPull(t *testing.T, titles []string, f FetchFunc, dir string, force bool) *PullReport {
	t.Helper()
	rep, err := Pull(titles, f, dir, "h", "/w/", force)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	return rep
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// --- tests ---

func TestPagePath(t *testing.T) {
	cases := []struct {
		title string
		ns    int
		want  string
	}{
		{"Open Source Water Pump", 0, "Open_Source_Water_Pump.wiki"},
		{"User:CameronBrooks11/wikisync", 2, "User/CameronBrooks11/wikisync.wiki"},
		{"User:CameronBrooks11", 2, "User/CameronBrooks11.wiki"},
		{"User talk:X", 3, "User_talk/X.wiki"},
		{"Foo/Bar", 0, "Foo/Bar.wiki"}, // main-namespace subpage
		{"Foo-Bar", 0, "Foo-Bar.wiki"},
		{"Help:A: B?", 12, "Help/A%3A_B%3F.wiki"},            // only the first colon is the namespace
		{"Notes: draft", 0, "Notes%3A_draft.wiki"},           // main namespace: a colon is just a character
		{"smw/schema:Thing", 112, "smw%2Fschema/Thing.wiki"}, // a namespace name with a slash stays one folder
		{"100%", 0, "100%25.wiki"},
	}
	for _, c := range cases {
		if got := PagePath(c.title, c.ns); got != c.want {
			t.Errorf("PagePath(%q, %d) = %q, want %q", c.title, c.ns, got, c.want)
		}
	}
}

// Titles come from a remote wiki; no title may produce a path outside the
// working tree. Anything that would falls back to one flat, encoded name.
func TestPagePathStaysInsideTheTree(t *testing.T) {
	cases := []struct {
		title string
		ns    int
		want  string
	}{
		{"../../etc/passwd", 0, "..%2F..%2Fetc%2Fpasswd.wiki"},
		{"A/../B", 0, "A%2F..%2FB.wiki"},
		{"A/./B", 0, "A%2F.%2FB.wiki"},
		{"A//B", 0, "A%2F%2FB.wiki"},
		{"A/", 0, "A%2F.wiki"},
		{"/A", 0, "%2FA.wiki"},
		{"..:x", 2, "..%3Ax.wiki"}, // a namespace named ".."
		{"User:..", 2, "User%3A...wiki"},
	}
	for _, c := range cases {
		got := PagePath(c.title, c.ns)
		if got != c.want {
			t.Errorf("PagePath(%q, %d) = %q, want %q", c.title, c.ns, got, c.want)
		}
		if !filepath.IsLocal(filepath.FromSlash(got)) {
			t.Errorf("PagePath(%q, %d) = %q escapes the working tree", c.title, c.ns, got)
		}
	}
}

func TestPullWritesNamespacedPageIntoFolders(t *testing.T) {
	dir := t.TempDir()
	p := page("User:CameronBrooks11/wikisync", "hello\n", 7, 1)
	p.NS = 2
	rep := mustPull(t, []string{p.Title}, fakeFetch(map[string]mw.FetchedPage{p.Title: p}), dir, false)

	if len(rep.Written) != 1 || rep.Written[0].File != "User/CameronBrooks11/wikisync.wiki" {
		t.Fatalf("written = %+v", rep.Written)
	}
	if got := mustRead(t, filepath.Join(dir, "User", "CameronBrooks11", "wikisync.wiki")); got != "hello\n" {
		t.Errorf("file = %q", got)
	}
	items, err := Status(dir)
	if err != nil || len(items) != 1 || items[0].State != Unchanged {
		t.Errorf("status = %+v, %v", items, err)
	}
}

// A page pulled under the old flat layout keeps its recorded path on re-pull,
// so nothing moves under the user.
func TestPullKeepsTrackedPagePath(t *testing.T) {
	dir := t.TempDir()
	const old = "User%3AX.wiki"
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{
		"User:X": {Title: "User:X", File: old, RevID: 1, Host: "h", Path: "/w/",
			SHA256: store.ContentHash("v1"), PageID: 3},
	}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, old), "v1")

	p := page("User:X", "v2", 3, 2)
	p.NS = 2
	rep := mustPull(t, []string{"User:X"}, fakeFetch(map[string]mw.FetchedPage{"User:X": p}), dir, false)

	if len(rep.Written) != 1 || rep.Written[0].File != old {
		t.Fatalf("written = %+v, want the recorded path %q", rep.Written, old)
	}
	if got := mustRead(t, filepath.Join(dir, old)); got != "v2" {
		t.Errorf("file = %q", got)
	}
	if fileExists(filepath.Join(dir, "User", "X.wiki")) {
		t.Error("a second copy was written at the new layout's path")
	}
}

// A main-namespace subpage and a namespaced page can map to the same path; the
// second one is refused rather than overwriting the first.
func TestPullRefusesLayoutCollision(t *testing.T) {
	dir := t.TempDir()
	main := page("User/foo", "main ns\n", 1, 1)
	user := page("User:foo", "user ns\n", 2, 1)
	user.NS = 2
	rep := mustPull(t, []string{"User/foo", "User:foo"},
		fakeFetch(map[string]mw.FetchedPage{"User/foo": main, "User:foo": user}), dir, false)

	if len(rep.Written) != 1 || len(rep.Collisions) != 1 || rep.Collisions[0].Owner != "User/foo" {
		t.Fatalf("rep = %+v", rep)
	}
	if got := mustRead(t, filepath.Join(dir, "User", "foo.wiki")); got != "main ns\n" {
		t.Errorf("first page was overwritten: %q", got)
	}
}

func TestPullWritesFilesBaselineAndManifest(t *testing.T) {
	dir := t.TempDir()
	rep := mustPull(t, []string{"Water Pump"},
		fakeFetch(map[string]mw.FetchedPage{"Water Pump": page("Water Pump", "== Build ==\n", 42, 7)}),
		dir, false)

	if len(rep.Written) != 1 || rep.Written[0].File != "Water_Pump.wiki" {
		t.Fatalf("written = %+v", rep.Written)
	}
	if got := mustRead(t, filepath.Join(dir, "Water_Pump.wiki")); got != "== Build ==\n" {
		t.Errorf("file = %q", got)
	}
	if _, ok, err := store.ReadBaseline(dir, 42); err != nil || !ok {
		t.Errorf("baseline not written (ok=%v err=%v)", ok, err)
	}
	m, err := store.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if e := m.Entries["Water Pump"]; e.RevID != 7 || e.PageID != 42 || e.SHA256 == "" {
		t.Errorf("entry = %+v", e)
	}
}

func TestPullMissingAndPartial(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]mw.FetchedPage{
		"Real":  page("Real", "x\n", 1, 1),
		"Ghost": {Title: "Ghost", Exists: false},
	}
	rep := mustPull(t, []string{"Real", "Ghost"}, fakeFetch(pages), dir, false)
	if len(rep.Written) != 1 || rep.Written[0].Title != "Real" {
		t.Errorf("written = %+v", rep.Written)
	}
	if len(rep.Missing) != 1 || rep.Missing[0] != "Ghost" {
		t.Errorf("missing = %+v", rep.Missing)
	}
}

func TestPullRefusesDirty(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "original\n", 1, 1)}), dir, false)
	mustWrite(t, filepath.Join(dir, "A.wiki"), "my local edit\n")

	rep := mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "newer\n", 1, 2)}), dir, false)
	if len(rep.SkippedDirty) != 1 || len(rep.Written) != 0 {
		t.Fatalf("rep = %+v", rep)
	}
	if got := mustRead(t, filepath.Join(dir, "A.wiki")); got != "my local edit\n" {
		t.Errorf("clobbered local edit: %q", got)
	}
}

func TestPullForceOverwritesDirty(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "original\n", 1, 1)}), dir, false)
	mustWrite(t, filepath.Join(dir, "A.wiki"), "my local edit\n")

	rep := mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "newer\n", 1, 2)}), dir, true)
	if len(rep.Written) != 1 {
		t.Fatalf("rep = %+v", rep)
	}
	if got := mustRead(t, filepath.Join(dir, "A.wiki")); got != "newer\n" {
		t.Errorf("not overwritten: %q", got)
	}
}

func TestPullRefreshesClean(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "v1\n", 1, 1)}), dir, false)
	rep := mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "v2\n", 1, 2)}), dir, false)

	if len(rep.Written) != 1 {
		t.Fatalf("rep = %+v", rep)
	}
	if got := mustRead(t, filepath.Join(dir, "A.wiki")); got != "v2\n" {
		t.Errorf("not refreshed: %q", got)
	}
	m, err := store.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Entries["A"].RevID != 2 {
		t.Errorf("revid = %d", m.Entries["A"].RevID)
	}
}

func TestPullDetectsCollision(t *testing.T) {
	dir := t.TempDir()
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{
		"Other": {Title: "Other", File: "Foo.wiki", RevID: 1, Host: "h", Path: "/w/", SHA256: "x", PageID: 9},
	}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	rep := mustPull(t, []string{"Foo"}, fakeFetch(map[string]mw.FetchedPage{"Foo": page("Foo", "x\n", 1, 1)}), dir, false)
	if len(rep.Collisions) != 1 || rep.Collisions[0].Owner != "Other" {
		t.Fatalf("collisions = %+v", rep.Collisions)
	}
	if len(rep.Written) != 0 {
		t.Error("should not have written over a collision")
	}
}
