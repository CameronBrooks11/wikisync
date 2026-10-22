package mw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testClient(srv *httptest.Server) *Client {
	return &Client{HTTP: srv.Client(), APIURL: srv.URL, UserAgent: "ua"}
}

func TestFetchPageReadsRevisionAndPageID(t *testing.T) {
	const body = `{"query":{"pages":[{"pageid":5,"title":"Water Pump",` +
		`"revisions":[{"revid":99,"slots":{"main":{"content":"hello\n"}}}]}]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("formatversion"); got != "2" {
			t.Errorf("formatversion = %q, want 2", got)
		}
		if got := r.URL.Query().Get("rvprop"); got != "ids|content" {
			t.Errorf("rvprop = %q, want ids|content", got)
		}
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	got, err := testClient(srv).FetchPage("Water Pump")
	if err != nil {
		t.Fatal(err)
	}
	want := FetchedPage{Title: "Water Pump", Text: "hello\n", RevID: 99, PageID: 5, Exists: true}
	if got != want {
		t.Errorf("FetchPage = %+v, want %+v", got, want)
	}
}

func TestFetchPageMissing(t *testing.T) {
	const body = `{"query":{"pages":[{"title":"Ghost","missing":true}]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	got, err := testClient(srv).FetchPage("Ghost")
	if err != nil {
		t.Fatal(err)
	}
	if got.Exists || got.RevID != 0 || got.PageID != 0 || got.Text != "" {
		t.Errorf("FetchPage = %+v, want missing", got)
	}
}

// fakeWiki routes the login/token/edit calls a push makes, recording the last
// edit it saw.
func TestLoginAndEdit(t *testing.T) {
	var gotText, gotSummary, gotBaseRevID, gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := r.FormValue("action")
		if action == "" {
			action = r.URL.Query().Get("action")
		}
		switch action {
		case "query": // meta=tokens
			typ := r.URL.Query().Get("type")
			_, _ = io.WriteString(w, `{"query":{"tokens":{"`+typ+`token":"TOK-`+typ+`"}}}`)
		case "login":
			if r.FormValue("lgtoken") != "TOK-login" {
				t.Errorf("login token = %q", r.FormValue("lgtoken"))
			}
			_, _ = io.WriteString(w, `{"login":{"result":"Success"}}`)
		case "edit":
			gotText = r.FormValue("text")
			gotSummary = r.FormValue("summary")
			gotBaseRevID = r.FormValue("baserevid")
			gotToken = r.FormValue("token")
			_, _ = io.WriteString(w, `{"edit":{"result":"Success","newrevid":1234}}`)
		default:
			t.Errorf("unexpected action %q", action)
		}
	}))
	defer srv.Close()

	c := testClient(srv)
	if err := c.Login("bot", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	newRev, err := c.Edit("Water Pump", "new text\n", "tidy", 99)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if newRev != 1234 {
		t.Errorf("newRev = %d, want 1234", newRev)
	}
	if gotText != "new text\n" || gotSummary != "tidy" || gotBaseRevID != "99" || gotToken != "TOK-csrf" {
		t.Errorf("edit params: text=%q summary=%q baserevid=%q token=%q", gotText, gotSummary, gotBaseRevID, gotToken)
	}
}

func TestEditReportsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.URL.Query().Get("action") == "query" {
			_, _ = io.WriteString(w, `{"query":{"tokens":{"csrftoken":"T"}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"error":{"code":"editconflict","info":"Edit conflict"}}`)
	}))
	defer srv.Close()

	_, err := testClient(srv).Edit("A", "x", "s", 1)
	if err == nil {
		t.Fatal("want an error on editconflict")
	}
}

// editServer answers token requests and returns body for the edit itself.
func editServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("action") == "query" {
			_, _ = io.WriteString(w, `{"query":{"tokens":{"csrftoken":"T"}}}`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
}

func TestEditNoChangeReturnsZero(t *testing.T) {
	srv := editServer(`{"edit":{"result":"Success","pageid":5,"title":"A","nochange":true}}`)
	defer srv.Close()

	rev, err := testClient(srv).Edit("A", "x", "s", 1)
	if err != nil || rev != 0 {
		t.Fatalf("Edit = (%d, %v), want (0, nil) for a null edit", rev, err)
	}
}

func TestEditSuccessWithoutRevisionIsAnError(t *testing.T) {
	srv := editServer(`{"edit":{"result":"Success","pageid":5,"title":"A"}}`)
	defer srv.Close()

	if _, err := testClient(srv).Edit("A", "x", "s", 1); err == nil {
		t.Fatal("want an error when success carries neither newrevid nor nochange")
	}
}

func TestRevisionText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("revids"); got != "1234" {
			t.Errorf("revids = %q, want 1234", got)
		}
		_, _ = io.WriteString(w, `{"query":{"pages":[{"pageid":5,"title":"A",`+
			`"revisions":[{"revid":1234,"slots":{"main":{"content":"stored text"}}}]}]}}`)
	}))
	defer srv.Close()

	text, err := testClient(srv).RevisionText(1234)
	if err != nil || text != "stored text" {
		t.Fatalf("RevisionText = (%q, %v)", text, err)
	}
}
