// Package mw is a small client for the MediaWiki JSON API: reading pages, and
// (for push) logging in and editing. It uses only the standard library.
package mw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// FetchedPage is a page's current wikitext and the revision it came from.
type FetchedPage struct {
	Title  string
	Text   string
	RevID  int64
	PageID int64
	Exists bool
}

// Client is a MediaWiki API session. A cookie jar carries the login session
// across requests, so Login must precede Edit.
type Client struct {
	HTTP      *http.Client
	APIURL    string
	UserAgent string
}

// APIURL builds the api.php endpoint, e.g.
// ("https", "www.appropedia.org", "/w/") -> "https://www.appropedia.org/w/api.php".
func APIURL(scheme, host, path string) string {
	return scheme + "://" + host + path + "api.php"
}

// NewClient returns a Client with its own cookie jar and a sane timeout.
func NewClient(host, path, userAgent string) *Client {
	jar, _ := cookiejar.New(nil) // cookiejar.New never returns an error
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second, Jar: jar},
		APIURL:    APIURL("https", host, path),
		UserAgent: userAgent,
	}
}

func (c *Client) get(params url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.APIURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	return c.HTTP.Do(req)
}

func (c *Client) post(params url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, c.APIURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.HTTP.Do(req)
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
// Content and revision id come from the same response, so a baseline's revid
// always matches its saved text. Missing pages come back with Exists == false.
func (c *Client) FetchPage(title string) (FetchedPage, error) {
	resp, err := c.get(url.Values{
		"action":        {"query"},
		"prop":          {"revisions"},
		"titles":        {title},
		"rvprop":        {"ids|content"},
		"rvslots":       {"main"},
		"rvlimit":       {"1"},
		"format":        {"json"},
		"formatversion": {"2"},
	})
	if err != nil {
		return FetchedPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()
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

// CurrentRevID returns a page's current remote revision id, for conflict checks.
func (c *Client) CurrentRevID(title string) (revid int64, exists bool, err error) {
	page, err := c.FetchPage(title)
	if err != nil {
		return 0, false, err
	}
	return page.RevID, page.Exists, nil
}

func (c *Client) token(tokenType string) (string, error) {
	resp, err := c.get(url.Values{
		"action":        {"query"},
		"meta":          {"tokens"},
		"type":          {tokenType},
		"format":        {"json"},
		"formatversion": {"2"},
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Query struct {
			Tokens map[string]string `json:"tokens"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	tok := out.Query.Tokens[tokenType+"token"]
	if tok == "" {
		return "", fmt.Errorf("no %s token in response", tokenType)
	}
	return tok, nil
}

// Login authenticates with a MediaWiki bot password; the session is held in the
// cookie jar for subsequent Edit calls.
func (c *Client) Login(username, password string) error {
	loginToken, err := c.token("login")
	if err != nil {
		return err
	}
	resp, err := c.post(url.Values{
		"action":        {"login"},
		"lgname":        {username},
		"lgpassword":    {password},
		"lgtoken":       {loginToken},
		"format":        {"json"},
		"formatversion": {"2"},
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Login struct {
			Result string `json:"result"`
			Reason string `json:"reason"`
		} `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Login.Result != "Success" {
		if out.Login.Reason != "" {
			return fmt.Errorf("login failed: %s (%s)", out.Login.Result, out.Login.Reason)
		}
		return fmt.Errorf("login failed: %s", out.Login.Result)
	}
	return nil
}

// Edit replaces a page's wikitext. baseRevID (when > 0) is passed to the API for
// server-side edit-conflict protection. Returns the new revision id.
func (c *Client) Edit(title, text, summary string, baseRevID int64) (int64, error) {
	csrf, err := c.token("csrf")
	if err != nil {
		return 0, err
	}
	params := url.Values{
		"action":        {"edit"},
		"title":         {title},
		"text":          {text},
		"summary":       {summary},
		"token":         {csrf},
		"bot":           {"1"},
		"format":        {"json"},
		"formatversion": {"2"},
	}
	if baseRevID > 0 {
		params.Set("baserevid", strconv.FormatInt(baseRevID, 10))
	}
	resp, err := c.post(params)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Edit struct {
			Result   string `json:"result"`
			NewRevID int64  `json:"newrevid"`
		} `json:"edit"`
		Error *struct {
			Code string `json:"code"`
			Info string `json:"info"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	if out.Error != nil {
		return 0, fmt.Errorf("edit error %s: %s", out.Error.Code, out.Error.Info)
	}
	if out.Edit.Result != "Success" {
		return 0, fmt.Errorf("edit not successful: %s", out.Edit.Result)
	}
	return out.Edit.NewRevID, nil
}
