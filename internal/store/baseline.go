package store

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

const BaselinesSubdir = "baselines"

// BaselinePath is where a page's immutable pulled-text snapshot lives. Keyed by
// page id so a wiki-side rename does not orphan the snapshot.
func BaselinePath(root string, pageID int64) string {
	return filepath.Join(root, ManifestDir, BaselinesSubdir, strconv.FormatInt(pageID, 10)+".wiki")
}

// WriteBaseline saves the snapshot of a page's pulled wikitext.
func WriteBaseline(root string, pageID int64, text string) error {
	path := BaselinePath(root, pageID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// ReadBaseline returns the snapshot text; ok is false when there is none
// (e.g. a pre-v2 pull that never wrote a baseline).
func ReadBaseline(root string, pageID int64) (text string, ok bool, err error) {
	data, err := os.ReadFile(BaselinePath(root, pageID))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}
