package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

func TestDiffUnchanged(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "hello\n", 1, 1)}), dir, false)

	res, err := Diff(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].State != DiffUnchanged {
		t.Fatalf("res = %+v", res)
	}
}

func TestDiffChanged(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "line1\nline2\n", 1, 1)}), dir, false)
	mustWrite(t, filepath.Join(dir, "A.wiki"), "line1\nCHANGED\n")

	res, err := Diff(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].State != Changed {
		t.Fatalf("state = %s", res[0].State)
	}
	if !strings.Contains(res[0].Text, "-line2") || !strings.Contains(res[0].Text, "+CHANGED") {
		t.Errorf("unexpected diff:\n%s", res[0].Text)
	}
}

func TestDiffDeleted(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "x\n", 1, 1)}), dir, false)
	if err := os.Remove(filepath.Join(dir, "A.wiki")); err != nil {
		t.Fatal(err)
	}

	res, err := Diff(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].State != DiffDeleted {
		t.Errorf("state = %s", res[0].State)
	}
}

func TestDiffNoBaseline(t *testing.T) {
	dir := t.TempDir()
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{
		"A": {Title: "A", File: "A.wiki", RevID: 1, Host: "h", Path: "/w/", SHA256: "x", PageID: 0},
	}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "A.wiki"), "x\n")

	res, err := Diff(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].State != NoBaseline {
		t.Errorf("state = %s", res[0].State)
	}
}

func TestDiffFilterByTitle(t *testing.T) {
	dir := t.TempDir()
	pages := map[string]mw.FetchedPage{"A": page("A", "a\n", 1, 1), "B": page("B", "b\n", 2, 1)}
	mustPull(t, []string{"A", "B"}, fakeFetch(pages), dir, false)
	mustWrite(t, filepath.Join(dir, "A.wiki"), "a2\n")

	res, err := Diff(dir, []string{"A"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Title != "A" {
		t.Fatalf("res = %+v", res)
	}
}
