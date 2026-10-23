// Package core holds wikisync's commands: pull, status and diff.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

// PagePath maps a page to its path in the working tree, relative to the root
// and slash-separated:
//
//	Water Pump                     -> Water_Pump.wiki
//	User:CameronBrooks11/wikisync  -> User/CameronBrooks11/wikisync.wiki
//	User talk:X                    -> User_talk/X.wiki
//
// A page outside the main namespace (ns != 0) goes in a folder named after its
// namespace, and subpage slashes become subfolders. Spaces become underscores
// (MediaWiki treats the two as the same title), and characters unsafe in
// filenames are percent-encoded within each segment, so distinct titles map to
// distinct paths.
//
// The title comes from the wiki, so the path is checked before use: if any
// segment would be empty, "." or "..", the page gets a single flat name with
// every slash encoded instead, which cannot leave the working tree.
func PagePath(title string, ns int) string {
	title = strings.ReplaceAll(title, " ", "_")
	folder, rest := "", title
	if ns != 0 {
		if i := strings.Index(title, ":"); i > 0 {
			folder, rest = title[:i], title[i+1:]
		}
	}

	var parts []string
	if folder != "" {
		parts = append(parts, encodeSegment(folder)) // e.g. "smw/schema" -> "smw%2Fschema"
	}
	parts = append(parts, strings.Split(rest, "/")...)
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return encodeSegment(title) + ".wiki"
		}
		if i > 0 || folder == "" {
			parts[i] = encodeSegment(part)
		}
	}
	// Every segment is now non-empty, not "." or "..", and has its slashes,
	// backslashes and colons encoded, so the joined path stays in the tree.
	return strings.Join(parts, "/") + ".wiki"
}

// encodeSegment percent-encodes the characters that are unsafe in a filename,
// including "/" and "%" itself, so the result is a single reversible segment.
func encodeSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		if isUnsafe(r) {
			fmt.Fprintf(&b, "%%%02X", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isUnsafe(r rune) bool {
	if r < 0x20 {
		return true
	}
	return strings.ContainsRune(`<>:"/\|?*%`, r)
}

// Collision reports a filename already owned by a different title.
type Collision struct {
	Title string
	File  string
	Owner string
}

// WrittenPage is a page pulled to disk.
type WrittenPage struct {
	Title string
	File  string
}

// PullReport summarises the outcome of a Pull.
type PullReport struct {
	Written      []WrittenPage
	Missing      []string
	SkippedDirty []string
	Collisions   []Collision
}

// FetchFunc fetches one page by title. It is injected so Pull is testable
// without a network.
type FetchFunc func(title string) (mw.FetchedPage, error)

// Pull fetches each title, writes its wikitext (and a baseline snapshot), and
// updates the manifest. It refuses to overwrite a local file that has unpulled
// changes or is untracked unless force is set; a clean tracked file is refreshed.
func Pull(titles []string, fetch FetchFunc, root, host, path string, force bool) (*PullReport, error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return nil, err
	}
	rep := &PullReport{}
	for _, title := range titles {
		page, err := fetch(title)
		if err != nil {
			return nil, err
		}
		if !page.Exists {
			rep.Missing = append(rep.Missing, title)
			continue
		}

		entry, tracked := m.Entries[page.Title]
		filename := PagePath(page.Title, page.NS)
		if tracked && entry.File != "" {
			filename = entry.File // a page already pulled keeps its path
		}
		dest := filepath.Join(root, filepath.FromSlash(filename))

		if owner := filenameOwner(m, filename, page.Title); owner != "" {
			rep.Collisions = append(rep.Collisions, Collision{Title: page.Title, File: filename, Owner: owner})
			continue
		}

		if fileExists(dest) && !force && !isClean(dest, entry, tracked) {
			rep.SkippedDirty = append(rep.SkippedDirty, page.Title)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, []byte(page.Text), 0o644); err != nil {
			return nil, err
		}
		if err := store.WriteBaseline(root, page.PageID, page.Text); err != nil {
			return nil, err
		}
		m.Entries[page.Title] = store.Entry{
			Title:  page.Title,
			File:   filename,
			RevID:  page.RevID,
			Host:   host,
			Path:   path,
			SHA256: store.ContentHash(page.Text),
			PageID: page.PageID,
		}
		rep.Written = append(rep.Written, WrittenPage{Title: page.Title, File: filename})
	}
	if err := m.Save(root); err != nil {
		return nil, err
	}
	return rep, nil
}

// filenameOwner returns a different title already mapped to filename, if any.
func filenameOwner(m *store.Manifest, filename, title string) string {
	for other, entry := range m.Entries {
		if entry.File == filename && other != title {
			return other
		}
	}
	return ""
}

// isClean reports whether the on-disk file matches its recorded baseline hash.
func isClean(dest string, entry store.Entry, tracked bool) bool {
	if !tracked || entry.SHA256 == "" {
		return false
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return false
	}
	return store.ContentHash(string(data)) == entry.SHA256
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
