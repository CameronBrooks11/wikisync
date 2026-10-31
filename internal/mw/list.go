package mw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Site is what wikisync needs from siteinfo: the wiki's content language and
// its namespace names (local, canonical and aliases) mapped to namespace ids.
type Site struct {
	Lang       string
	Namespaces map[string]int // keyed by normalizeNS(name)
}

// normalizeNS makes namespace names comparable: case-insensitive, with
// underscores and spaces treated alike, as MediaWiki does.
func normalizeNS(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", " "))
}

// SiteInfo fetches the wiki's content language and namespaces once per Client.
func (c *Client) SiteInfo() (Site, error) {
	if c.site != nil {
		return *c.site, nil
	}
	resp, err := c.get(url.Values{
		"action":        {"query"},
		"meta":          {"siteinfo"},
		"siprop":        {"general|namespaces|namespacealiases"},
		"format":        {"json"},
		"formatversion": {"2"},
	})
	if err != nil {
		return Site{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Site{}, fmt.Errorf("mediawiki api: %s", resp.Status)
	}

	var out struct {
		Error *apiError `json:"error"`
		Query struct {
			General struct {
				Lang string `json:"lang"`
			} `json:"general"`
			Namespaces map[string]struct {
				ID        int    `json:"id"`
				Name      string `json:"name"`
				Canonical string `json:"canonical"`
			} `json:"namespaces"`
			NamespaceAliases []struct {
				ID    int    `json:"id"`
				Alias string `json:"alias"`
			} `json:"namespacealiases"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Site{}, err
	}
	if out.Error != nil {
		return Site{}, out.Error
	}
	site := Site{Lang: out.Query.General.Lang, Namespaces: map[string]int{}}
	for _, ns := range out.Query.Namespaces {
		for _, name := range []string{ns.Name, ns.Canonical} {
			if name != "" {
				site.Namespaces[normalizeNS(name)] = ns.ID
			}
		}
	}
	for _, a := range out.Query.NamespaceAliases {
		site.Namespaces[normalizeNS(a.Alias)] = a.ID
	}
	c.site = &site
	return site, nil
}

// ContentLanguage is the wiki's own language, e.g. "en".
func (c *Client) ContentLanguage() (string, error) {
	site, err := c.SiteInfo()
	if err != nil {
		return "", err
	}
	return site.Lang, nil
}

// ListedPage is a page found by a listing, with its page language (which marks
// translations, e.g. "es" on an English wiki) but not its text.
type ListedPage struct {
	Title string
	NS    int
	Lang  string
}

// CategoryMembers lists the pages (not subcategories or files) in a category.
// "Water" and "Category:Water" both work.
func (c *Client) CategoryMembers(category string) ([]ListedPage, error) {
	if !strings.HasPrefix(strings.ToLower(category), "category:") {
		category = "Category:" + category
	}
	return c.listGenerator(url.Values{
		"generator": {"categorymembers"},
		"gcmtitle":  {category},
		"gcmtype":   {"page"},
		"gcmlimit":  {"max"},
	})
}

// PrefixPages lists the pages whose title starts with prefix, e.g.
// "User:CameronBrooks11/". The namespace is resolved from the wiki's own
// namespace names; a prefix without a known namespace searches the main one.
func (c *Client) PrefixPages(prefix string) ([]ListedPage, error) {
	site, err := c.SiteInfo()
	if err != nil {
		return nil, err
	}
	ns, rest := 0, prefix
	if i := strings.Index(prefix, ":"); i > 0 {
		if id, ok := site.Namespaces[normalizeNS(prefix[:i])]; ok {
			ns, rest = id, prefix[i+1:]
		}
	}
	return c.listGenerator(url.Values{
		"generator":    {"allpages"},
		"gapnamespace": {strconv.Itoa(ns)},
		"gapprefix":    {rest},
		"gaplimit":     {"max"},
	})
}

// listGenerator runs a page generator with prop=info, following continuation,
// and returns the pages sorted by title.
func (c *Client) listGenerator(params url.Values) ([]ListedPage, error) {
	params.Set("action", "query")
	params.Set("prop", "info")
	params.Set("format", "json")
	params.Set("formatversion", "2")

	var pages []ListedPage
	cont := map[string]string{}
	for {
		q := url.Values{}
		for k, v := range params {
			q[k] = v
		}
		for k, v := range cont {
			q.Set(k, v)
		}
		batch, next, err := c.listOnce(q)
		if err != nil {
			return nil, err
		}
		pages = append(pages, batch...)
		if len(next) == 0 {
			break
		}
		cont = next
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Title < pages[j].Title })
	return pages, nil
}

func (c *Client) listOnce(q url.Values) ([]ListedPage, map[string]string, error) {
	resp, err := c.get(q)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("mediawiki api: %s", resp.Status)
	}

	var out struct {
		Error    *apiError         `json:"error"`
		Continue map[string]string `json:"continue"`
		Query    struct {
			Pages []struct {
				Title        string `json:"title"`
				NS           int    `json:"ns"`
				PageLanguage string `json:"pagelanguage"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, nil, err
	}
	if out.Error != nil {
		return nil, nil, out.Error
	}
	pages := make([]ListedPage, 0, len(out.Query.Pages))
	for _, p := range out.Query.Pages {
		pages = append(pages, ListedPage{Title: p.Title, NS: p.NS, Lang: p.PageLanguage})
	}
	return pages, out.Continue, nil
}

// batchSize is MediaWiki's per-request title limit for ordinary accounts.
const batchSize = 50

// FetchPages fetches the current text of many pages, batchSize per request.
// The result is keyed by the title as requested; MediaWiki's normalized form
// (e.g. underscores to spaces) is mapped back. Pages that don't exist come back
// with Exists == false.
func (c *Client) FetchPages(titles []string) (map[string]FetchedPage, error) {
	result := make(map[string]FetchedPage, len(titles))
	for start := 0; start < len(titles); start += batchSize {
		batch := titles[start:min(start+batchSize, len(titles))]
		if err := c.fetchBatch(batch, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *Client) fetchBatch(batch []string, result map[string]FetchedPage) error {
	byTitle := map[string]FetchedPage{}
	normalized := map[string]string{}
	cont := map[string]string{}
	for {
		next, added, err := c.fetchBatchOnce(batch, cont, byTitle, normalized)
		if err != nil {
			return err
		}
		if len(next) == 0 {
			break
		}
		// Over the response size limit, MediaWiki leaves some pages' revisions
		// out and says where to continue; each round must deliver some.
		if added == 0 && len(cont) > 0 {
			return fmt.Errorf("mediawiki api: a continued request returned no page text")
		}
		cont = next
	}
	for _, requested := range batch {
		key := requested
		if to, ok := normalized[requested]; ok {
			key = to
		}
		page, ok := byTitle[key]
		if !ok {
			page = FetchedPage{Title: requested, Exists: false}
		}
		result[requested] = page
	}
	return nil
}

// fetchBatchOnce makes one request for batch (continuing from cont), records
// pages with text or known to be missing in byTitle, and returns the next
// continuation and how many pages gained their text.
func (c *Client) fetchBatchOnce(batch []string, cont map[string]string, byTitle map[string]FetchedPage, normalized map[string]string) (map[string]string, int, error) {
	params := url.Values{
		"action":        {"query"},
		"prop":          {"revisions"},
		"titles":        {strings.Join(batch, "|")},
		"rvprop":        {"ids|content"},
		"rvslots":       {"main"},
		"format":        {"json"},
		"formatversion": {"2"},
	}
	for k, v := range cont {
		params.Set(k, v)
	}
	// POST: fifty long titles can exceed a URL length limit.
	resp, err := c.post(params)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("mediawiki api: %s", resp.Status)
	}

	var out struct {
		Error    *apiError         `json:"error"`
		Continue map[string]string `json:"continue"`
		Query    struct {
			Normalized []struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"normalized"`
			Pages []struct {
				PageID    int64  `json:"pageid"`
				NS        int    `json:"ns"`
				Title     string `json:"title"`
				Missing   bool   `json:"missing"`
				Invalid   bool   `json:"invalid"`
				Revisions []struct {
					RevID int64 `json:"revid"`
					Slots struct {
						Main struct {
							Content string `json:"content"`
						} `json:"main"`
					} `json:"slots"`
				} `json:"revisions"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, 0, err
	}
	if out.Error != nil {
		return nil, 0, out.Error
	}

	for _, n := range out.Query.Normalized {
		normalized[n.From] = n.To
	}
	added := 0
	for _, p := range out.Query.Pages {
		switch {
		case p.Missing || p.Invalid:
			byTitle[p.Title] = FetchedPage{Title: p.Title, Exists: false}
		case len(p.Revisions) > 0:
			if _, done := byTitle[p.Title]; !done {
				added++
			}
			rev := p.Revisions[0]
			byTitle[p.Title] = FetchedPage{
				Title:  p.Title,
				Text:   rev.Slots.Main.Content,
				RevID:  rev.RevID,
				PageID: p.PageID,
				NS:     p.NS,
				Exists: true,
			}
		}
	}
	return out.Continue, added, nil
}
