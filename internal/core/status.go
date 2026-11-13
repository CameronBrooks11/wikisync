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
	titles, _ := m.Select(nil)
	items := make([]StatusItem, 0, len(titles))
	for _, title := range titles {
		entry := m.Entries[title]
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.File)))
		var state State
		switch {
		case errors.Is(err, fs.ErrNotExist):
			state = Deleted
		case entry.SHA256 == "":
			state = BaselineUnknown
		case err != nil:
			return nil, err
		case store.ContentHash(string(data)) == entry.SHA256:
			state = Unchanged
		default:
			state = Modified
		}
		items = append(items, StatusItem{Title: title, File: entry.File, State: state})
	}
	return items, nil
}
