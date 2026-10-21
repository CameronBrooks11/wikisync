// Package store persists what wikisync pulled: the manifest and the immutable
// baseline snapshots, under the .wikisync directory of a working tree.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	ManifestDir    = ".wikisync"
	ManifestFile   = "manifest.json"
	CurrentVersion = 2 // v2 added Entry.PageID and baseline snapshots
)

// Entry records one pulled page: where it lives locally and the revision it tracks.
type Entry struct {
	Title  string `json:"title"`
	File   string `json:"file"`
	RevID  int64  `json:"revid"`
	Host   string `json:"host"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"` // hash of the pulled wikitext ("" for pre-hash manifests)
	PageID int64  `json:"pageid"` // MediaWiki page id (0 for pre-v2 entries)
}

// Manifest is the on-disk record of a working tree, keyed by page title.
type Manifest struct {
	Version int              `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

// ContentHash is a stable hash of wikitext, used to detect local edits.
func ContentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func manifestPath(root string) string {
	return filepath.Join(root, ManifestDir, ManifestFile)
}

// LoadManifest reads the manifest, returning an empty one if none exists.
// Unknown fields from a newer schema are ignored so an old binary fails soft.
func LoadManifest(root string) (*Manifest, error) {
	data, err := os.ReadFile(manifestPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{Version: CurrentVersion, Entries: map[string]Entry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Entries == nil {
		m.Entries = map[string]Entry{}
	}
	return &m, nil
}

// Save writes the manifest atomically (temp file + rename), upgrading the schema.
func (m *Manifest) Save(root string) error {
	dir := filepath.Join(root, ManifestDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m.Version = CurrentVersion
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := filepath.Join(dir, ManifestFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestPath(root)) // atomic on the same filesystem
}
