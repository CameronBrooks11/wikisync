package core

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// State classifies a tracked file against its recorded baseline.
type State string

const (
	Unchanged       State = "unchanged"
	Modified        State = "modified"
	Deleted         State = "deleted"
	BaselineUnknown State = "baseline-unknown"
)

// StatusItem is one page's state.
type StatusItem struct {
	Title string
	File  string
	State State
}

// Status classifies each tracked page's local file against its baseline hash.
// It is local only and never contacts the wiki.
func Status(root string) ([]StatusItem, error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return nil, err
	}
	items := make([]StatusItem, 0, len(m.Entries))
	for title, entry := range m.Entries {
		dest := filepath.Join(root, entry.File)
		var state State
		switch {
		case !fileExists(dest):
			state = Deleted
		case entry.SHA256 == "":
			state = BaselineUnknown
		default:
			data, err := os.ReadFile(dest)
			if err != nil {
				return nil, err
			}
			if store.ContentHash(string(data)) == entry.SHA256 {
				state = Unchanged
			} else {
				state = Modified
			}
		}
		items = append(items, StatusItem{Title: title, File: entry.File, State: state})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	return items, nil
}
