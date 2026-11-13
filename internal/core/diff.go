package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// DiffState describes what a page's diff produced.
type DiffState string

const (
	Changed       DiffState = "changed"
	DiffUnchanged DiffState = "unchanged"
	DiffDeleted   DiffState = "deleted"
	NoBaseline    DiffState = "baseline-unavailable"
)

// PageDiff is one page's diff result; Text holds the unified diff when Changed.
type PageDiff struct {
	Title string
	State DiffState
	Text  string
}

// Diff compares each tracked page (or the named ones) to its baseline snapshot
// and returns a unified diff; the names root doesn't track come back as
// untracked. It is local only and never contacts the wiki.
func Diff(root string, titles []string) (results []PageDiff, untracked []string, err error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return nil, nil, err
	}
	keys, untracked := m.Select(titles)

	results = make([]PageDiff, 0, len(keys))
	for _, title := range keys {
		entry := m.Entries[title]
		if entry.PageID == 0 {
			results = append(results, PageDiff{Title: title, State: NoBaseline})
			continue
		}
		baseline, ok, err := store.ReadBaseline(root, entry.PageID)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			results = append(results, PageDiff{Title: title, State: NoBaseline})
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.File)))
		if errors.Is(err, fs.ErrNotExist) {
			results = append(results, PageDiff{Title: title, State: DiffDeleted})
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		local := string(data)
		if local == baseline {
			results = append(results, PageDiff{Title: title, State: DiffUnchanged})
			continue
		}
		text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(baseline),
			B:        difflib.SplitLines(local),
			FromFile: fmt.Sprintf("a/%s (rev %d)", entry.File, entry.RevID),
			ToFile:   fmt.Sprintf("b/%s (local)", entry.File),
			Context:  3,
		})
		if err != nil {
			return nil, nil, err
		}
		results = append(results, PageDiff{Title: title, State: Changed, Text: text})
	}
	return results, untracked, nil
}
