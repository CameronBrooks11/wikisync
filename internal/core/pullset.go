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
// Categories are followed Depth levels into their subcategories (0: the
// category's own pages only). With nothing named and no selectors, every
// tracked page is pulled again.
type Selection struct {
	Titles       []string
	Categories   []string
	Depth        int
	Prefixes     []string
	AllLanguages bool
	Max          int
}

// Lister finds pages for a selection; listed pages carry their page language.
type Lister interface {
	ContentLanguage() (string, error)
	CategoryMembers(category string) (pages []mw.ListedPage, subcats []string, err error)
	PrefixPages(prefix string) ([]mw.ListedPage, error)
}

// BatchFetchFunc fetches many pages at once, keyed by the title as requested.
type BatchFetchFunc func(titles []string) (map[string]mw.FetchedPage, error)

// SetReport is a PullReport plus what selection left out.
type SetReport struct {
	PullReport
	SkippedLanguage  []string // listed pages in another language (e.g. translations)
	Refreshed        bool     // nothing was named, so every tracked page was pulled again
	CategoriesWalked int      // categories listed, subcategories included
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

// categoryKey compares category names: "Water", "Category:Water" and
// "category:Water" name the same category.
func categoryKey(c string) string {
	c = titleKey(strings.TrimSpace(c))
	if strings.HasPrefix(strings.ToLower(c), "category:") {
		c = strings.TrimSpace(c[len("category:"):])
	}
	return c
}

// categoryPages lists the pages in each category and in its subcategories down
// to depth levels, breadth first. Categories can contain each other, and a
// subcategory can be reached by several paths, so each is listed once. It
// returns the pages (possibly with repeats across categories) and how many
// categories were listed.
func categoryPages(list Lister, roots []string, depth int) ([]mw.ListedPage, int, error) {
	visited := map[string]bool{}
	var level []string
	for _, c := range roots {
		if !visited[categoryKey(c)] {
			visited[categoryKey(c)] = true
			level = append(level, c)
		}
	}
	var pages []mw.ListedPage
	walked := 0
	for d := 0; len(level) > 0; d++ {
		var next []string
		for _, c := range level {
			ps, subcats, err := list.CategoryMembers(c)
			if err != nil {
				return nil, 0, fmt.Errorf("category %q: %w", c, err)
			}
			walked++
			pages = append(pages, ps...)
			if d == depth {
				continue
			}
			for _, s := range subcats {
				if !visited[categoryKey(s)] {
					visited[categoryKey(s)] = true
					next = append(next, s)
				}
			}
		}
		level = next
	}
	return pages, walked, nil
}

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
		listed, walked, err := categoryPages(list, sel.Categories, sel.Depth)
		if err != nil {
			return nil, err
		}
		rep.CategoriesWalked = walked
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
