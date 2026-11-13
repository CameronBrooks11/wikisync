package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// State classifies a tracked file against its recorded baseline.
type State string

const (
	StatusUnchanged       State = "unchanged"
	StatusModified        State = "modified"
	StatusDeleted         State = "deleted"
	StatusBaselineUnknown State = "baseline-unknown"
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
	titles, _ := m.Select(nil)
	items := make([]StatusItem, 0, len(titles))
	for _, title := range titles {
		entry := m.Entries[title]
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.File)))
		var state State
		switch {
		case errors.Is(err, fs.ErrNotExist):
			state = StatusDeleted
		case entry.SHA256 == "":
			state = StatusBaselineUnknown
		case err != nil:
			return nil, err
		case store.ContentHash(string(data)) == entry.SHA256:
			state = StatusUnchanged
		default:
			state = StatusModified
		}
		items = append(items, StatusItem{Title: title, File: entry.File, State: state})
	}
	return items, nil
}
