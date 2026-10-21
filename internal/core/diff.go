package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
// and returns a unified diff. It is local only and never contacts the wiki.
func Diff(root string, titles []string) ([]PageDiff, error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return nil, err
	}

	selected := map[string]store.Entry{}
	if titles == nil {
		selected = m.Entries
	} else {
		for _, t := range titles {
			if entry, ok := m.Entries[t]; ok {
				selected[t] = entry
			}
		}
	}

	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	results := make([]PageDiff, 0, len(keys))
	for _, title := range keys {
		entry := selected[title]
		if entry.PageID == 0 {
			results = append(results, PageDiff{Title: title, State: NoBaseline})
			continue
		}
		baseline, ok, err := store.ReadBaseline(root, entry.PageID)
		if err != nil {
			return nil, err
		}
		if !ok {
			results = append(results, PageDiff{Title: title, State: NoBaseline})
			continue
		}
		dest := filepath.Join(root, entry.File)
		if !fileExists(dest) {
			results = append(results, PageDiff{Title: title, State: DiffDeleted})
			continue
		}
		data, err := os.ReadFile(dest)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		results = append(results, PageDiff{Title: title, State: Changed, Text: text})
	}
	return results, nil
}
