package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

// Selection is what a pull covers. Titles named explicitly are always pulled.
// Pages found through Categories and Prefixes are limited to the wiki's own
// language unless AllLanguages is set, and refused if more than Max remain.
// With nothing named and no selectors, every tracked page is pulled again.
type Selection struct {
	Titles       []string
	Categories   []string
	Prefixes     []string
	AllLanguages bool
	Max          int
}

// Lister finds pages for a selection; listed pages carry their page language.
type Lister interface {
	ContentLanguage() (string, error)
	CategoryMembers(category string) ([]mw.ListedPage, error)
	PrefixPages(prefix string) ([]mw.ListedPage, error)
}

// BatchFetchFunc fetches many pages at once, keyed by the title as requested.
type BatchFetchFunc func(titles []string) (map[string]mw.FetchedPage, error)

// SetReport is a PullReport plus what selection left out.
type SetReport struct {
	PullReport
	SkippedLanguage []string // listed pages in another language (e.g. translations)
	Refreshed       bool     // nothing was named, so every tracked page was pulled again
}

// ErrNothingToPull means no titles or selectors were given and nothing is tracked.
var ErrNothingToPull = errors.New("nothing to pull: name pages, or use --category or --prefix")

// TooManyError reports a category/prefix selection larger than Selection.Max.
type TooManyError struct {
	Count, Max int
}

func (e *TooManyError) Error() string {
	return fmt.Sprintf("the selection has %d pages, more than the limit of %d (raise it with --max)", e.Count, e.Max)
}

// titleKey compares titles the way MediaWiki does for spaces and underscores.
func titleKey(t string) string { return strings.ReplaceAll(t, "_", " ") }

// PullSet resolves a selection, fetches its pages in batches, and pulls them
// with Pull, so dirty files, collisions and baselines are handled the same way.
func PullSet(sel Selection, list Lister, fetch BatchFetchFunc, root, host, path string, force bool) (*SetReport, error) {
	rep := &SetReport{}

	var explicit []string
	seen := map[string]bool{}
	for _, t := range sel.Titles {
		if !seen[titleKey(t)] {
			seen[titleKey(t)] = true
			explicit = append(explicit, t)
		}
	}

	var selected []string
	switch {
	case len(sel.Categories) > 0 || len(sel.Prefixes) > 0:
		lang := ""
		if !sel.AllLanguages {
			var err error
			if lang, err = list.ContentLanguage(); err != nil {
				return nil, err
			}
		}
		var listed []mw.ListedPage
		for _, c := range sel.Categories {
			pages, err := list.CategoryMembers(c)
			if err != nil {
				return nil, fmt.Errorf("category %q: %w", c, err)
			}
			listed = append(listed, pages...)
		}
		for _, p := range sel.Prefixes {
			pages, err := list.PrefixPages(p)
			if err != nil {
				return nil, fmt.Errorf("prefix %q: %w", p, err)
			}
			listed = append(listed, pages...)
		}
		for _, p := range listed {
			if seen[titleKey(p.Title)] {
				continue
			}
			seen[titleKey(p.Title)] = true
			if lang != "" && p.Lang != "" && p.Lang != lang {
				rep.SkippedLanguage = append(rep.SkippedLanguage, p.Title)
				continue
			}
			selected = append(selected, p.Title)
		}
		sort.Strings(selected)
		sort.Strings(rep.SkippedLanguage)
		if sel.Max > 0 && len(selected) > sel.Max {
			return nil, &TooManyError{Count: len(selected), Max: sel.Max}
		}

	case len(explicit) == 0:
		m, err := store.LoadManifest(root)
		if err != nil {
			return nil, err
		}
		for t := range m.Entries {
			explicit = append(explicit, t)
		}
		if len(explicit) == 0 {
			return nil, ErrNothingToPull
		}
		sort.Strings(explicit)
		rep.Refreshed = true
	}

	titles := append(explicit, selected...)
	if len(titles) == 0 {
		return rep, nil // the selectors matched nothing
	}
	pages, err := fetch(titles)
	if err != nil {
		return nil, err
	}
	pr, err := Pull(titles, func(t string) (mw.FetchedPage, error) {
		if p, ok := pages[t]; ok {
			return p, nil
		}
		return mw.FetchedPage{Title: t, Exists: false}, nil
	}, root, host, path, force)
	if err != nil {
		return nil, err
	}
	rep.PullReport = *pr
	return rep, nil
}
