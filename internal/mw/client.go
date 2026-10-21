// Package mw is a thin read client for the MediaWiki JSON API.
//
// Writing (push) will grow here once implemented; for now this is read-only and
// uses only the standard library.
package mw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FetchedPage is a page's current wikitext and the revision it came from.
type FetchedPage struct {
	Title  string
	Text   string
	RevID  int64
	PageID int64
	Exists bool
}

// APIURL builds the api.php endpoint from a scheme, host and script path, e.g.
// ("https", "www.appropedia.org", "/w/") -> "https://www.appropedia.org/w/api.php".
func APIURL(scheme, host, path string) string {
	return scheme + "://" + host + path + "api.php"
}

// apiResponse mirrors a formatversion=2 query/revisions response.
type apiResponse struct {
	Query struct {
		Pages []struct {
			PageID    int64  `json:"pageid"`
			Title     string `json:"title"`
			Missing   bool   `json:"missing"`
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

// FetchPage returns a page's current wikitext and the revision it came from.
//
// Content and revision id are read from the same revisions response, so the
// recorded baseline revid always matches the saved text. Missing pages come back
// with Exists == false.
func FetchPage(client *http.Client, apiURL, userAgent, title string) (FetchedPage, error) {
	q := url.Values{
		"action":        {"query"},
		"prop":          {"revisions"},
		"titles":        {title},
		"rvprop":        {"ids|content"},
		"rvslots":       {"main"},
		"rvlimit":       {"1"},
		"format":        {"json"},
		"formatversion": {"2"},
	}
	req, err := http.NewRequest(http.MethodGet, apiURL+"?"+q.Encode(), nil)
	if err != nil {
		return FetchedPage{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return FetchedPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FetchedPage{}, fmt.Errorf("mediawiki api: %s", resp.Status)
	}

	var parsed apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return FetchedPage{}, err
	}
	if len(parsed.Query.Pages) == 0 {
		return FetchedPage{}, fmt.Errorf("no page in response for %q", title)
	}
	page := parsed.Query.Pages[0]
	if page.Missing {
		return FetchedPage{Title: page.Title, Exists: false}, nil
	}
	if len(page.Revisions) == 0 {
		return FetchedPage{}, fmt.Errorf("page %q has no revisions", title)
	}
	rev := page.Revisions[0]
	return FetchedPage{
		Title:  page.Title,
		Text:   rev.Slots.Main.Content,
		RevID:  rev.RevID,
		PageID: page.PageID,
		Exists: true,
	}, nil
}
