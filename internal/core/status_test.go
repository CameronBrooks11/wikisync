package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
)

func TestStatusUnchangedAfterPull(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "hello\n", 1, 1)}), dir, false)

	items, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != Unchanged {
		t.Fatalf("items = %+v", items)
	}
}

func TestStatusModified(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "hello\n", 1, 1)}), dir, false)
	mustWrite(t, filepath.Join(dir, "A.wiki"), "changed\n")

	items, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].State != Modified {
		t.Errorf("state = %s", items[0].State)
	}
}

func TestStatusDeleted(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "x\n", 1, 1)}), dir, false)
	if err := os.Remove(filepath.Join(dir, "A.wiki")); err != nil {
		t.Fatal(err)
	}

	items, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].State != Deleted {
		t.Errorf("state = %s", items[0].State)
	}
}

func TestStatusEmpty(t *testing.T) {
	items, err := Status(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("want empty, got %+v", items)
	}
}
