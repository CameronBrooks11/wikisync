package mw

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const siteinfoJSON = `{"query":{"general":{"lang":"en"},"namespaces":{` +
	`"0":{"id":0,"name":""},` +
	`"2":{"id":2,"name":"User","canonical":"User"},` +
	`"3":{"id":3,"name":"User talk","canonical":"User talk"},` +
	`"14":{"id":14,"name":"Category","canonical":"Category"}},` +
	`"namespacealiases":[{"id":4,"alias":"WP"}]}}`

func TestSiteInfoParsesAndCaches(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = io.WriteString(w, siteinfoJSON)
	}))
	defer srv.Close()

	c := testClient(srv)
	site, err := c.SiteInfo()
	if err != nil {
		t.Fatal(err)
	}
	if site.Lang != "en" || site.Namespaces["user"] != 2 || site.Namespaces["user talk"] != 3 || site.Namespaces["wp"] != 4 {
		t.Errorf("site = %+v", site)
	}
	if lang, _ := c.ContentLanguage(); lang != "en" || calls != 1 {
		t.Errorf("lang = %q after %d requests, want one cached request", lang, calls)
	}
}

func TestCategoryMembersFollowsContinuation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("gcmtitle") != "Category:Water" || q.Get("gcmtype") != "page" || q.Get("prop") != "info" {
			t.Errorf("query = %v", q)
		}
		if q.Get("gcmcontinue") == "" {
			_, _ = io.WriteString(w, `{"continue":{"gcmcontinue":"next","continue":"gcmcontinue||"},`+
				`"query":{"pages":[{"title":"B","ns":0,"pagelanguage":"en"},{"title":"A/es","ns":0,"pagelanguage":"es"}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"query":{"pages":[{"title":"C","ns":0,"pagelanguage":"en"}]}}`)
	}))
	defer srv.Close()

	pages, err := testClient(srv).CategoryMembers("Water")
	if err != nil {
		t.Fatal(err)
	}
	want := []ListedPage{{"A/es", 0, "es"}, {"B", 0, "en"}, {"C", 0, "en"}}
	if fmt.Sprint(pages) != fmt.Sprint(want) {
		t.Errorf("pages = %v, want %v", pages, want)
	}
}

func TestPrefixPagesResolvesNamespace(t *testing.T) {
	cases := []struct{ prefix, ns, rest string }{
		{"User:CameronBrooks11/", "2", "CameronBrooks11/"},
		{"user_talk:X", "3", "X"},
		{"WP:Rules", "4", "Rules"},
		{"Notes: draft", "0", "Notes: draft"}, // not a namespace: main namespace
		{"Water", "0", "Water"},
	}
	for _, tc := range cases {
		var gotNS, gotRest string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("meta") == "siteinfo" {
				_, _ = io.WriteString(w, siteinfoJSON)
				return
			}
			gotNS, gotRest = q.Get("gapnamespace"), q.Get("gapprefix")
			_, _ = io.WriteString(w, `{"query":{"pages":[]}}`)
		}))
		if _, err := testClient(srv).PrefixPages(tc.prefix); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		if gotNS != tc.ns || gotRest != tc.rest {
			t.Errorf("PrefixPages(%q): namespace %q prefix %q, want %q %q", tc.prefix, gotNS, gotRest, tc.ns, tc.rest)
		}
	}
}

// The fake echoes each requested title back as a page, normalizing
// underscores to spaces the way MediaWiki does, and reports "Ghost" missing.
func batchServer(t *testing.T, requests *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		*requests++
		_ = r.ParseForm()
		titles := strings.Split(r.FormValue("titles"), "|")
		if len(titles) > batchSize {
			t.Errorf("batch of %d titles, over the limit of %d", len(titles), batchSize)
		}
		type rev struct {
			RevID int64                        `json:"revid"`
			Slots map[string]map[string]string `json:"slots"`
		}
		var normalized []map[string]string
		var pages []map[string]any
		for i, title := range titles {
			norm := strings.ReplaceAll(title, "_", " ")
			if norm != title {
				normalized = append(normalized, map[string]string{"from": title, "to": norm})
			}
			if norm == "Ghost" {
				pages = append(pages, map[string]any{"title": norm, "missing": true})
				continue
			}
			pages = append(pages, map[string]any{
				"pageid": i + 1, "ns": 0, "title": norm,
				"revisions": []rev{{RevID: 100, Slots: map[string]map[string]string{"main": {"content": "text of " + norm}}}},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"query": map[string]any{"normalized": normalized, "pages": pages}})
	}))
}

func TestFetchPagesBatchesAndMapsTitles(t *testing.T) {
	requests := 0
	srv := batchServer(t, &requests)
	defer srv.Close()

	titles := []string{"Water_Pump", "Ghost"}
	for i := 0; i < 49; i++ {
		titles = append(titles, fmt.Sprintf("P%d", i))
	}
	got, err := testClient(srv).FetchPages(titles) // 51 titles -> 2 requests
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
	if len(got) != len(titles) {
		t.Fatalf("got %d pages for %d titles", len(got), len(titles))
	}
	if p := got["Water_Pump"]; !p.Exists || p.Title != "Water Pump" || p.Text != "text of Water Pump" || p.RevID != 100 {
		t.Errorf("Water_Pump = %+v (keyed by the requested title, normalized title inside)", p)
	}
	if p := got["Ghost"]; p.Exists {
		t.Errorf("Ghost = %+v, want missing", p)
	}
}
