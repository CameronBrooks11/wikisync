package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

type fakeLister struct {
	lang       string
	categories map[string][]mw.ListedPage // keyed by name without "Category:"
	subcats    map[string][]string        // keyed the same way
	prefixes   map[string][]mw.ListedPage
	listed     *[]string // categories listed, in order, when non-nil
}

func (f fakeLister) ContentLanguage() (string, error) { return f.lang, nil }

// CategoryMembers accepts "Water" and "Category:Water", as MediaWiki does.
func (f fakeLister) CategoryMembers(c string) ([]mw.ListedPage, []string, error) {
	name := strings.TrimPrefix(c, "Category:")
	if f.listed != nil {
		*f.listed = append(*f.listed, name)
	}
	return f.categories[name], f.subcats[name], nil
}
func (f fakeLister) PrefixPages(p string) ([]mw.ListedPage, error) { return f.prefixes[p], nil }

// fakeBatch serves pages from a map and records each call's titles.
type fakeBatch struct {
	pages map[string]mw.FetchedPage
	calls [][]string
}

func (f *fakeBatch) fetch(titles []string) (map[string]mw.FetchedPage, error) {
	f.calls = append(f.calls, append([]string(nil), titles...))
	out := map[string]mw.FetchedPage{}
	for _, t := range titles {
		// Like MediaWiki, "Water_Pump" and "Water Pump" are the same page.
		for key, p := range f.pages {
			if titleKey(key) == titleKey(t) {
				out[t] = p
			}
		}
	}
	return out, nil
}

func written(rep *SetReport) []string {
	var titles []string
	for _, w := range rep.Written {
		titles = append(titles, w.Title)
	}
	return titles
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A category with English pages and bot translations, as on Appropedia.
func waterWiki() (fakeLister, *fakeBatch) {
	list := fakeLister{lang: "en", categories: map[string][]mw.ListedPage{
		"Water": {
			{Title: "Still", Lang: "en"},
			{Title: "Still/es", Lang: "es"},
			{Title: "Pump", Lang: "en"},
			{Title: "Pump/ar", Lang: "ar"},
		},
	}}
	b := &fakeBatch{pages: map[string]mw.FetchedPage{
		"Still":    page("Still", "s\n", 1, 1),
		"Still/es": page("Still/es", "es\n", 2, 1),
		"Pump":     page("Pump", "p\n", 3, 1),
		"Pump/ar":  page("Pump/ar", "ar\n", 4, 1),
	}}
	return list, b
}

func TestPullSetCategorySkipsTranslations(t *testing.T) {
	list, b := waterWiki()
	rep, err := PullSet(Selection{Categories: []string{"Water"}, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := written(rep); !equal(got, []string{"Pump", "Still"}) {
		t.Errorf("written = %v", got)
	}
	if !equal(rep.SkippedLanguage, []string{"Pump/ar", "Still/es"}) {
		t.Errorf("skipped = %v", rep.SkippedLanguage)
	}
	if len(b.calls) != 1 {
		t.Errorf("fetch calls = %d, want one batch", len(b.calls))
	}
}

func TestPullSetAllLanguages(t *testing.T) {
	list, b := waterWiki()
	rep, err := PullSet(Selection{Categories: []string{"Water"}, AllLanguages: true, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := written(rep); len(got) != 4 || len(rep.SkippedLanguage) != 0 {
		t.Errorf("written = %v, skipped = %v", got, rep.SkippedLanguage)
	}
}

// A page named explicitly is pulled whatever its language, and only once.
func TestPullSetExplicitTitleAlwaysPulled(t *testing.T) {
	list, b := waterWiki()
	rep, err := PullSet(Selection{Titles: []string{"Still/es"}, Categories: []string{"Water"}, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := written(rep); !equal(got, []string{"Still/es", "Pump", "Still"}) {
		t.Errorf("written = %v", got)
	}
	if !equal(rep.SkippedLanguage, []string{"Pump/ar"}) {
		t.Errorf("skipped = %v", rep.SkippedLanguage)
	}
}

// The limit counts pages after translations are left out, and nothing is
// fetched when it is exceeded.
func TestPullSetMaxCountsAfterLanguageFilter(t *testing.T) {
	list, b := waterWiki()
	if _, err := PullSet(Selection{Categories: []string{"Water"}, Max: 2}, list, b.fetch, t.TempDir(), "h", "/w/", false); err != nil {
		t.Fatalf("2 English pages under a limit of 2: %v", err)
	}

	b.calls = nil
	_, err := PullSet(Selection{Categories: []string{"Water"}, Max: 1}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	var tooMany *TooManyError
	if !errors.As(err, &tooMany) || tooMany.Count != 2 || tooMany.Max != 1 {
		t.Fatalf("err = %v, want TooManyError{2, 1}", err)
	}
	if len(b.calls) != 0 {
		t.Error("fetched pages despite refusing the selection")
	}
}

// "Water_Pump" named and "Water Pump" listed are the same page.
func TestPullSetDeduplicatesUnderscores(t *testing.T) {
	list := fakeLister{lang: "en", prefixes: map[string][]mw.ListedPage{
		"Water": {{Title: "Water Pump", Lang: "en"}},
	}}
	b := &fakeBatch{pages: map[string]mw.FetchedPage{"Water_Pump": page("Water Pump", "x\n", 1, 1)}}
	rep, err := PullSet(Selection{Titles: []string{"Water_Pump"}, Prefixes: []string{"Water"}, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := written(rep); len(got) != 1 {
		t.Errorf("written = %v, want the page once", got)
	}
	if len(b.calls) != 1 || len(b.calls[0]) != 1 {
		t.Errorf("fetched %v, want the page requested once", b.calls)
	}
}

// Nothing named: every tracked page is pulled again.
func TestPullSetRefreshesTrackedPages(t *testing.T) {
	dir := t.TempDir()
	mustPull(t, []string{"A", "B"}, fakeFetch(map[string]mw.FetchedPage{
		"A": page("A", "a1\n", 1, 1), "B": page("B", "b1\n", 2, 1),
	}), dir, false)

	b := &fakeBatch{pages: map[string]mw.FetchedPage{"A": page("A", "a2\n", 1, 2), "B": page("B", "b2\n", 2, 2)}}
	rep, err := PullSet(Selection{Max: 500}, fakeLister{}, b.fetch, dir, "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Refreshed || !equal(written(rep), []string{"A", "B"}) {
		t.Fatalf("rep = %+v", rep)
	}
	m, _ := store.LoadManifest(dir)
	if m.Entries["A"].RevID != 2 || m.Entries["B"].RevID != 2 {
		t.Errorf("manifest not refreshed: %+v", m.Entries)
	}
}

func TestPullSetNothingToPull(t *testing.T) {
	_, err := PullSet(Selection{Max: 500}, fakeLister{}, (&fakeBatch{}).fetch, t.TempDir(), "h", "/w/", false)
	if !errors.Is(err, ErrNothingToPull) {
		t.Fatalf("err = %v, want ErrNothingToPull", err)
	}
}

func TestPullSetSelectorMatchingNothing(t *testing.T) {
	b := &fakeBatch{}
	rep, err := PullSet(Selection{Categories: []string{"Empty"}, Max: 500}, fakeLister{lang: "en"}, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Written) != 0 || len(b.calls) != 0 {
		t.Errorf("rep = %+v, calls = %v", rep, b.calls)
	}
}

// A category tree like Appropedia's: Lab has two subcategories that share a
// third (Hardware), Hardware links back to Lab, and one page sits in two
// categories. Subcategory names come back as "Category:…", as from the API.
func labWiki() (fakeLister, *fakeBatch, *[]string) {
	listed := &[]string{}
	list := fakeLister{lang: "en", listed: listed,
		categories: map[string][]mw.ListedPage{
			"Lab":       {{Title: "Lab page", Lang: "en"}},
			"Methods":   {{Title: "Method", Lang: "en"}, {Title: "Method/es", Lang: "es"}},
			"Completed": {{Title: "Project", Lang: "en"}, {Title: "Method", Lang: "en"}},
			"Hardware":  {{Title: "Device", Lang: "en"}},
		},
		subcats: map[string][]string{
			"Lab":       {"Category:Methods", "Category:Completed"},
			"Methods":   {"Category:Hardware"},
			"Completed": {"Category:Hardware"},
			"Hardware":  {"Category:Lab"},
		},
	}
	b := &fakeBatch{pages: map[string]mw.FetchedPage{}}
	for i, t := range []string{"Lab page", "Method", "Method/es", "Project", "Device"} {
		b.pages[t] = page(t, t+"\n", int64(i+1), 1)
	}
	return list, b, listed
}

func TestPullSetDepthFollowsSubcategories(t *testing.T) {
	cases := []struct {
		depth  int
		pages  []string
		listed []string
	}{
		{0, []string{"Lab page"}, []string{"Lab"}},
		{1, []string{"Lab page", "Method", "Project"}, []string{"Lab", "Methods", "Completed"}},
		{2, []string{"Device", "Lab page", "Method", "Project"}, []string{"Lab", "Methods", "Completed", "Hardware"}},
		// Hardware -> Lab is a loop: the walk ends once every category is seen.
		{9, []string{"Device", "Lab page", "Method", "Project"}, []string{"Lab", "Methods", "Completed", "Hardware"}},
	}
	for _, tc := range cases {
		list, b, listed := labWiki()
		rep, err := PullSet(Selection{Categories: []string{"Lab"}, Depth: tc.depth, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
		if err != nil {
			t.Fatalf("depth %d: %v", tc.depth, err)
		}
		if got := written(rep); !equal(got, tc.pages) {
			t.Errorf("depth %d: written = %v, want %v", tc.depth, got, tc.pages)
		}
		if !equal(*listed, tc.listed) || rep.CategoriesWalked != len(tc.listed) {
			t.Errorf("depth %d: listed %v (walked %d), want %v", tc.depth, *listed, rep.CategoriesWalked, tc.listed)
		}
	}
}

// One category named twice, in different spellings, is listed once.
func TestPullSetCategorySpellings(t *testing.T) {
	list, b, listed := labWiki()
	_, err := PullSet(Selection{Categories: []string{"Lab", "Category:Lab", "category:Lab"}, Max: 500}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(*listed, []string{"Lab"}) {
		t.Errorf("listed %v, want Lab once", *listed)
	}
}

// The limit counts the whole tree, after translations in subcategories are
// left out, and nothing is fetched when it is exceeded.
func TestPullSetMaxCountsWholeTree(t *testing.T) {
	list, b, _ := labWiki()
	_, err := PullSet(Selection{Categories: []string{"Lab"}, Depth: 2, Max: 3}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	var tooMany *TooManyError
	if !errors.As(err, &tooMany) || tooMany.Count != 4 {
		t.Fatalf("err = %v, want TooManyError counting 4 pages", err)
	}
	if len(b.calls) != 0 {
		t.Error("fetched pages despite refusing the selection")
	}

	rep, err := PullSet(Selection{Categories: []string{"Lab"}, Depth: 2, Max: 4}, list, b.fetch, t.TempDir(), "h", "/w/", false)
	if err != nil {
		t.Fatalf("4 pages under a limit of 4: %v", err)
	}
	if !equal(rep.SkippedLanguage, []string{"Method/es"}) {
		t.Errorf("skipped = %v", rep.SkippedLanguage)
	}
}
