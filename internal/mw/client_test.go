package mw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	got, err := FetchPage(srv.Client(), srv.URL, "ua", "Water Pump")
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

	got, err := FetchPage(srv.Client(), srv.URL, "ua", "Ghost")
	if err != nil {
		t.Fatal(err)
	}
	if got.Exists || got.RevID != 0 || got.PageID != 0 || got.Text != "" {
		t.Errorf("FetchPage = %+v, want missing", got)
	}
}
