package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Version: CurrentVersion, Entries: map[string]Entry{}}
	entry := Entry{Title: "Water Pump", File: "Water_Pump.wiki", RevID: 7, Host: "h", Path: "/w/", SHA256: "abc", PageID: 3}
	m.Entries["Water Pump"] = entry
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", loaded.Version, CurrentVersion)
	}
	if loaded.Entries["Water Pump"] != entry {
		t.Errorf("entry = %+v, want %+v", loaded.Entries["Water Pump"], entry)
	}
}

func TestLoadManifestMissingIsEmpty(t *testing.T) {
	m, err := LoadManifest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entries) != 0 {
		t.Errorf("want empty, got %+v", m.Entries)
	}
}

func TestLoadManifestV1WithoutPageID(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ManifestDir), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"entries":{"A":{"title":"A","file":"A.wiki","revid":3,"host":"h","path":"/w/","sha256":"abc"}}}`
	if err := os.WriteFile(filepath.Join(dir, ManifestDir, ManifestFile), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 {
		t.Errorf("version = %d, want 1", m.Version)
	}
	if got := m.Entries["A"]; got.PageID != 0 || got.SHA256 != "abc" {
		t.Errorf("entry = %+v", got)
	}
}
