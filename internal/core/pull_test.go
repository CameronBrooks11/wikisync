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

func TestTitleToFilename(t *testing.T) {
	cases := map[string]string{
		"Open Source Water Pump": "Open_Source_Water_Pump.wiki",
		"Foo/Bar":                "Foo%2FBar.wiki",
		"Foo-Bar":                "Foo-Bar.wiki",
		"A B":                    "A_B.wiki",
	}
	for in, want := range cases {
		if got := TitleToFilename(in); got != want {
			t.Errorf("TitleToFilename(%q) = %q, want %q", in, got, want)
		}
	}
	if TitleToFilename("Foo/Bar") == TitleToFilename("Foo-Bar") {
		t.Error("distinct titles must not collide")
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
