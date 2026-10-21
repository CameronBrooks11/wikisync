package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

type editCall struct {
	title, text, summary string
	baseRevID            int64
}

type fakeRemote struct {
	revs    map[string]int64 // current remote revid per title
	missing map[string]bool  // titles that no longer exist
	edits   []editCall
	editErr error
}

func (f *fakeRemote) CurrentRevID(title string) (int64, bool, error) {
	if f.missing[title] {
		return 0, false, nil
	}
	return f.revs[title], true, nil
}

func (f *fakeRemote) Edit(title, text, summary string, baseRevID int64) (int64, error) {
	if f.editErr != nil {
		return 0, f.editErr
	}
	f.edits = append(f.edits, editCall{title, text, summary, baseRevID})
	return baseRevID + 1, nil
}

// setup pulls page A at rev 5 (pageid 1), then applies the given local edit.
func setupModified(t *testing.T, localText string) string {
	t.Helper()
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", "orig\n", 1, 5)}), dir, false)
	if localText != "" {
		mustWrite(t, filepath.Join(dir, "A.wiki"), localText)
	}
	return dir
}

func TestPushPreviewWouldPush(t *testing.T) {
	dir := setupModified(t, "new\n")
	remote := &fakeRemote{revs: map[string]int64{"A": 5}} // remote still at baseline

	results, err := Push(dir, nil, remote, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != WouldPush {
		t.Fatalf("results = %+v", results)
	}
	if len(remote.edits) != 0 {
		t.Error("preview must not edit")
	}
}

func TestPushPreviewConflict(t *testing.T) {
	dir := setupModified(t, "new\n")
	remote := &fakeRemote{revs: map[string]int64{"A": 9}} // remote moved past baseline 5

	results, err := Push(dir, nil, remote, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != Conflict {
		t.Fatalf("action = %s", results[0].Action)
	}
}

func TestPushSkipsUnchanged(t *testing.T) {
	dir := setupModified(t, "") // no local edit
	remote := &fakeRemote{revs: map[string]int64{"A": 5}}

	results, err := Push(dir, nil, remote, true, "s")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != SkipUnchanged {
		t.Fatalf("action = %s", results[0].Action)
	}
	if len(remote.edits) != 0 {
		t.Error("unchanged page must not edit")
	}
}

func TestPushApplyEditsAndUpdatesBaseline(t *testing.T) {
	dir := setupModified(t, "new\n")
	remote := &fakeRemote{revs: map[string]int64{"A": 5}}

	results, err := Push(dir, nil, remote, true, "tidy")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != Pushed || results[0].NewRevID != 6 {
		t.Fatalf("result = %+v", results[0])
	}
	if len(remote.edits) != 1 || remote.edits[0].baseRevID != 5 || remote.edits[0].text != "new\n" {
		t.Fatalf("edit = %+v", remote.edits)
	}
	// Baseline advanced to the pushed text, and the manifest to the new revision.
	baseline, _, _ := store.ReadBaseline(dir, 1)
	if baseline != "new\n" {
		t.Errorf("baseline = %q, want new", baseline)
	}
	m, _ := store.LoadManifest(dir)
	if m.Entries["A"].RevID != 6 {
		t.Errorf("manifest revid = %d, want 6", m.Entries["A"].RevID)
	}
}

func TestPushApplyConflictDoesNotEdit(t *testing.T) {
	dir := setupModified(t, "new\n")
	remote := &fakeRemote{revs: map[string]int64{"A": 9}}

	results, err := Push(dir, nil, remote, true, "s")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != Conflict || len(remote.edits) != 0 {
		t.Fatalf("results = %+v edits = %+v", results, remote.edits)
	}
}

func TestPushEditErrorIsReported(t *testing.T) {
	dir := setupModified(t, "new\n")
	remote := &fakeRemote{revs: map[string]int64{"A": 5}, editErr: errors.New("boom")}

	results, err := Push(dir, nil, remote, true, "s")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != PushError {
		t.Fatalf("action = %s", results[0].Action)
	}
	// Baseline unchanged after a failed edit.
	baseline, _, _ := store.ReadBaseline(dir, 1)
	if baseline != "orig\n" {
		t.Errorf("baseline moved despite failed edit: %q", baseline)
	}
}

func TestPushDeletedLocalFileIsErrorNotRemoteDelete(t *testing.T) {
	dir := setupModified(t, "new\n")
	if err := os.Remove(filepath.Join(dir, "A.wiki")); err != nil {
		t.Fatal(err)
	}
	remote := &fakeRemote{revs: map[string]int64{"A": 5}}

	results, err := Push(dir, nil, remote, true, "s")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != PushError {
		t.Fatalf("action = %s (a deleted local file must not trigger a remote edit)", results[0].Action)
	}
	if len(remote.edits) != 0 {
		t.Error("must not edit for a missing local file")
	}
}
