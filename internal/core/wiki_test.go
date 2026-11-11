package core

import (
	"errors"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// tree writes a manifest tracking one page per host/path pair.
func tree(t *testing.T, wikis ...[2]string) string {
	t.Helper()
	dir := t.TempDir()
	m := &store.Manifest{Version: store.CurrentVersion, Entries: map[string]store.Entry{}}
	for i, w := range wikis {
		title := string(rune('A' + i))
		m.Entries[title] = store.Entry{Title: title, File: title + ".wiki", Host: w[0], Path: w[1]}
	}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveWikiEmptyTreeNeedsHost(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ResolveWiki(dir, "", "", "/w/"); !errors.Is(err, ErrNoHost) {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
	host, path, err := ResolveWiki(dir, "wiki.example.org", "", "/w/")
	if err != nil || host != "wiki.example.org" || path != "/w/" {
		t.Errorf("got %q %q %v, want wiki.example.org /w/", host, path, err)
	}
	if _, path, _ := ResolveWiki(dir, "wiki.example.org", "/", "/w/"); path != "/" {
		t.Errorf("path = %q, want the --path given", path)
	}
}

func TestResolveWikiUsesRecordedWiki(t *testing.T) {
	dir := tree(t, [2]string{"wiki.example.org", "/mw/"}, [2]string{"wiki.example.org", "/mw/"})
	host, path, err := ResolveWiki(dir, "", "", "/w/")
	if err != nil || host != "wiki.example.org" || path != "/mw/" {
		t.Errorf("got %q %q %v, want the recorded wiki", host, path, err)
	}
	if _, _, err := ResolveWiki(dir, "wiki.example.org", "/mw/", "/w/"); err != nil {
		t.Errorf("repeating the recorded wiki: %v", err)
	}
}

func TestResolveWikiRefusesAnotherWiki(t *testing.T) {
	dir := tree(t, [2]string{"wiki.example.org", "/w/"})
	var mismatch *WikiMismatchError
	if _, _, err := ResolveWiki(dir, "other.example.org", "", "/w/"); !errors.As(err, &mismatch) {
		t.Errorf("other host: err = %v, want WikiMismatchError", err)
	}
	if _, _, err := ResolveWiki(dir, "", "/mw/", "/w/"); !errors.As(err, &mismatch) {
		t.Errorf("other path: err = %v, want WikiMismatchError", err)
	}
}

// Trees made before 0.3.0 could mix wikis; --host then picks one.
func TestResolveWikiMixedTree(t *testing.T) {
	dir := tree(t, [2]string{"a.example.org", "/w/"}, [2]string{"b.example.org", "/w/"})
	if _, _, err := ResolveWiki(dir, "", "", "/w/"); err == nil {
		t.Error("no --host in a mixed tree: want an error")
	}
	if host, _, err := ResolveWiki(dir, "b.example.org", "", "/w/"); err != nil || host != "b.example.org" {
		t.Errorf("got %q %v, want b.example.org", host, err)
	}
	var mismatch *WikiMismatchError
	if _, _, err := ResolveWiki(dir, "c.example.org", "", "/w/"); !errors.As(err, &mismatch) {
		t.Errorf("unknown host: err = %v, want WikiMismatchError", err)
	}
}
