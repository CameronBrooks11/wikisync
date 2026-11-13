// Package mw is a small client for the MediaWiki JSON API: reading pages, and
// (for push) logging in and editing. It uses only the standard library.
package mw

import (
	"encoding/json"
	"fmt"
	"io"
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
	NS     int // namespace number (0 is the main namespace)
	Exists bool
}

// Client is a MediaWiki API session. A cookie jar carries the login session
// across requests, so Login must precede Edit.
type Client struct {
	HTTP      *http.Client
	APIURL    string
	UserAgent string

	site *Site // siteinfo, fetched once (see SiteInfo)
}

// APIURL builds the api.php endpoint, e.g.
// ("https", "wiki.example.org", "/w/") -> "https://wiki.example.org/w/api.php".
func APIURL(scheme, host, path string) string {
	return scheme + "://" + host + path + "api.php"
}

// NewClient returns a Client for an HTTPS wiki at host+path, with its own
// cookie jar and a sane timeout.
func NewClient(host, path, userAgent string) *Client {
	return NewClientForAPI(APIURL("https", host, path), userAgent)
}

// NewClientForAPI is NewClient for a full api.php URL, e.g. a local test wiki
// at http://localhost:8080/api.php.
func NewClientForAPI(apiURL, userAgent string) *Client {
	jar, _ := cookiejar.New(nil) // cookiejar.New never returns an error
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second, Jar: jar},
		APIURL:    apiURL,
		UserAgent: userAgent,
	}
}

// apiError is MediaWiki's error envelope: {"error":{"code":…,"info":…}}.
type apiError struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

func (e *apiError) Error() string { return "mediawiki api error " + e.Code + ": " + e.Info }

// call sends one API request as a GET or POST and decodes the response into
// out. A status other than 200 and MediaWiki's error envelope both come back
// as errors.
func (c *Client) call(method string, params url.Values, out any) error {
	params.Set("format", "json")
	params.Set("formatversion", "2")
	var req *http.Request
	var err error
	if method == http.MethodPost {
		req, err = http.NewRequest(method, c.APIURL, strings.NewReader(params.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	} else {
		req, err = http.NewRequest(method, c.APIURL+"?"+params.Encode(), nil)
	}
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mediawiki api: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope struct {
		Error *apiError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	return json.Unmarshal(body, out)
}

// revisionsResponse is a prop=revisions query: page texts, with the title
// normalisations and continuation a batch request can carry.
type revisionsResponse struct {
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

// CurrentRevID returns a page's current remote revision id, for conflict
// checks. It asks for the id alone, not the page text.
func (c *Client) CurrentRevID(title string) (revid int64, exists bool, err error) {
	var parsed revisionsResponse
	if err := c.call(http.MethodGet, url.Values{
		"action": {"query"},
		"prop":   {"revisions"},
		"titles": {title},
		"rvprop": {"ids"},
	}, &parsed); err != nil {
		return 0, false, err
	}
	if len(parsed.Query.Pages) == 0 {
		return 0, false, fmt.Errorf("no page in response for %q", title)
	}
	page := parsed.Query.Pages[0]
	if page.Missing {
		return 0, false, nil
	}
	if len(page.Revisions) == 0 {
		return 0, false, fmt.Errorf("page %q has no revisions", title)
	}
	return page.Revisions[0].RevID, true, nil
}

func (c *Client) token(tokenType string) (string, error) {
	var out struct {
		Query struct {
			Tokens map[string]string `json:"tokens"`
		} `json:"query"`
	}
	if err := c.call(http.MethodGet, url.Values{
		"action": {"query"},
		"meta":   {"tokens"},
		"type":   {tokenType},
	}, &out); err != nil {
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
	var out struct {
		Login struct {
			Result string `json:"result"`
			Reason string `json:"reason"`
		} `json:"login"`
	}
	if err := c.call(http.MethodPost, url.Values{
		"action":     {"login"},
		"lgname":     {username},
		"lgpassword": {password},
		"lgtoken":    {loginToken},
	}, &out); err != nil {
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
// server-side edit-conflict protection. Returns the new revision id, or 0 when
// the wiki made no change (a null edit, reported by the API as "nochange").
func (c *Client) Edit(title, text, summary string, baseRevID int64) (int64, error) {
	csrf, err := c.token("csrf")
	if err != nil {
		return 0, err
	}
	params := url.Values{
		"action":  {"edit"},
		"title":   {title},
		"text":    {text},
		"summary": {summary},
		"token":   {csrf},
		"bot":     {"1"},
	}
	if baseRevID > 0 {
		params.Set("baserevid", strconv.FormatInt(baseRevID, 10))
	}
	var out struct {
		Edit struct {
			Result   string `json:"result"`
			NewRevID int64  `json:"newrevid"`
			NoChange bool   `json:"nochange"`
		} `json:"edit"`
	}
	if err := c.call(http.MethodPost, params, &out); err != nil {
		return 0, err
	}
	if out.Edit.Result != "Success" {
		return 0, fmt.Errorf("edit not successful: %s", out.Edit.Result)
	}
	if out.Edit.NoChange {
		return 0, nil
	}
	if out.Edit.NewRevID == 0 {
		return 0, fmt.Errorf("edit reported success without a new revision id")
	}
	return out.Edit.NewRevID, nil
}

// RevisionText returns the wikitext the wiki stored for one revision.
func (c *Client) RevisionText(revID int64) (string, error) {
	var parsed revisionsResponse
	if err := c.call(http.MethodGet, url.Values{
		"action":  {"query"},
		"prop":    {"revisions"},
		"revids":  {strconv.FormatInt(revID, 10)},
		"rvprop":  {"ids|content"},
		"rvslots": {"main"},
	}, &parsed); err != nil {
		return "", err
	}
	for _, page := range parsed.Query.Pages {
		for _, rev := range page.Revisions {
			if rev.RevID == revID {
				return rev.Slots.Main.Content, nil
			}
		}
	}
	return "", fmt.Errorf("revision %d not found", revID)
}
