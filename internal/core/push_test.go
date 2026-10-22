package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CameronBrooks11/wikisync/internal/mw"
	"github.com/CameronBrooks11/wikisync/internal/store"
)

// fakeWiki mimics the MediaWiki behaviours push depends on:
//   - saving trims trailing whitespace (MediaWiki's pre-save transform);
//   - an edit that leaves the stored text unchanged creates no revision, and
//     the API reports it with no new revision id (newRevID 0);
//   - an edit whose base revision is not the current one is an edit conflict.
type fakeWiki struct {
	text    string
	rev     int64
	missing bool
	history map[int64]string // revid -> stored text
	next    int64
	created int // revisions actually created by Edit

	editErr     error
	readErr     error // RevisionText fails (e.g. a network blip after the edit)
	moveOnEdit  bool  // someone else edits just before ours lands
	edits       []string
	baseRevSeen []int64
}

func newFakeWiki(text string, rev int64) *fakeWiki {
	return &fakeWiki{text: text, rev: rev, history: map[int64]string{rev: text}, next: rev}
}

func (w *fakeWiki) save(text string) {
	w.next += 10
	w.rev = w.next
	w.text = text
	w.history[w.rev] = text
}

func (w *fakeWiki) CurrentRevID(string) (int64, bool, error) {
	if w.missing {
		return 0, false, nil
	}
	return w.rev, true, nil
}

func (w *fakeWiki) Edit(_, text, _ string, baseRevID int64) (int64, error) {
	if w.editErr != nil {
		return 0, w.editErr
	}
	w.edits = append(w.edits, text)
	w.baseRevSeen = append(w.baseRevSeen, baseRevID)
	if w.moveOnEdit {
		w.save(w.text + "\nsomeone else's edit")
	}
	if baseRevID > 0 && baseRevID != w.rev {
		return 0, errors.New("edit error editconflict: Edit conflict.")
	}
	stored := strings.TrimRight(text, " \t\n\r\x00\x0B")
	if stored == w.text {
		return 0, nil // null edit: no revision
	}
	w.save(stored)
	w.created++
	return w.rev, nil
}

func (w *fakeWiki) RevisionText(revID int64) (string, error) {
	if w.readErr != nil {
		return "", w.readErr
	}
	text, ok := w.history[revID]
	if !ok {
		return "", errors.New("no such revision")
	}
	return text, nil
}

// setupPulled pulls page A (pageid 1) at rev 5 with the wiki's stored text, then
// writes localText over the working file when it is non-empty.
func setupPulled(t *testing.T, wiki *fakeWiki, localText string) string {
	t.Helper()
	dir := t.TempDir()
	mustPull(t, []string{"A"}, fakeFetch(map[string]mw.FetchedPage{"A": page("A", wiki.text, 1, wiki.rev)}), dir, false)
	if localText != "" {
		mustWrite(t, filepath.Join(dir, "A.wiki"), localText)
	}
	return dir
}

func mustPush(t *testing.T, dir string, remote Remote, apply bool) []PushResult {
	t.Helper()
	results, err := Push(dir, nil, remote, apply, "summary")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v, want one", results)
	}
	return results
}

// assertInSync checks the invariant push must keep: manifest revision,
// baseline, working file and status all agree with what the wiki stores.
func assertInSync(t *testing.T, dir string, wiki *fakeWiki) {
	t.Helper()
	m, err := store.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := m.Entries["A"]
	if entry.RevID != wiki.rev {
		t.Errorf("manifest revid = %d, wiki revid = %d", entry.RevID, wiki.rev)
	}
	if baseline, _, _ := store.ReadBaseline(dir, 1); baseline != wiki.text {
		t.Errorf("baseline = %q, wiki stores %q", baseline, wiki.text)
	}
	if local := mustRead(t, filepath.Join(dir, "A.wiki")); local != wiki.text {
		t.Errorf("local file = %q, wiki stores %q", local, wiki.text)
	}
	items, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != Unchanged {
		t.Errorf("status = %+v, want unchanged", items)
	}
}

func TestPushPreviewWouldPush(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")

	if r := mustPush(t, dir, wiki, false); r[0].Action != WouldPush {
		t.Fatalf("action = %s", r[0].Action)
	}
	if len(wiki.edits) != 0 {
		t.Error("preview must not edit")
	}
}

func TestPushPreviewConflict(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	wiki.save("orig\nremote change") // the wiki moved past our baseline

	if r := mustPush(t, dir, wiki, false); r[0].Action != Conflict {
		t.Fatalf("action = %s", r[0].Action)
	}
}

func TestPushSkipsUnchanged(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "")

	if r := mustPush(t, dir, wiki, true); r[0].Action != SkipUnchanged {
		t.Fatalf("action = %s", r[0].Action)
	}
	if len(wiki.edits) != 0 {
		t.Error("unchanged page must not edit")
	}
}

// The wiki trims the trailing newline we send; the baseline and working file
// must record what it stored, not what we sent.
func TestPushRecordsWhatTheWikiStored(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew\n")

	r := mustPush(t, dir, wiki, true)
	if r[0].Action != Pushed || r[0].NewRevID != wiki.rev {
		t.Fatalf("result = %+v, wiki rev %d", r[0], wiki.rev)
	}
	if wiki.baseRevSeen[0] != 5 {
		t.Errorf("edit base revision = %d, want 5", wiki.baseRevSeen[0])
	}
	assertInSync(t, dir, wiki)

	// Pushing again is a no-op.
	if r := mustPush(t, dir, wiki, true); r[0].Action != SkipUnchanged {
		t.Errorf("second push action = %s, want unchanged", r[0].Action)
	}
	if wiki.created != 1 {
		t.Errorf("revisions created = %d, want 1", wiki.created)
	}
}

// A whitespace-only change (e.g. an editor adding a final newline) is a null
// edit on the wiki. It must not record a bogus revision.
func TestPushWhitespaceOnlyChangeIsNullEdit(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\n")

	r := mustPush(t, dir, wiki, true)
	if r[0].Action == PushError || r[0].Action == Conflict {
		t.Fatalf("result = %+v", r[0])
	}
	if wiki.created != 0 {
		t.Errorf("revisions created = %d, want 0", wiki.created)
	}
	assertInSync(t, dir, wiki)
}

// After a null edit, a later real edit must still go through rather than be
// refused as a conflict.
func TestPushAfterNullEditStillWorks(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\n")
	mustPush(t, dir, wiki, true)

	mustWrite(t, filepath.Join(dir, "A.wiki"), "orig\nreal change")
	if r := mustPush(t, dir, wiki, true); r[0].Action != Pushed {
		t.Fatalf("result = %+v, want pushed", r[0])
	}
	assertInSync(t, dir, wiki)
}

// The edit lands but reading the revision back fails: the new revision id must
// still be recorded, so the next push is not refused as a false conflict.
func TestPushReadBackFailureStillRecordsRevision(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	wiki.readErr = errors.New("network blip")

	r := mustPush(t, dir, wiki, true)
	if r[0].Action != Pushed || r[0].Detail == "" {
		t.Fatalf("result = %+v, want pushed with a warning", r[0])
	}
	if m, _ := store.LoadManifest(dir); m.Entries["A"].RevID != wiki.rev {
		t.Fatalf("manifest revid = %d, wiki revid = %d", m.Entries["A"].RevID, wiki.rev)
	}

	wiki.readErr = nil
	mustWrite(t, filepath.Join(dir, "A.wiki"), "orig\nnew\nmore")
	if r := mustPush(t, dir, wiki, true); r[0].Action != Pushed {
		t.Errorf("next push = %+v, want pushed (no false conflict)", r[0])
	}
}

func TestPushApplyConflictDoesNotEdit(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	wiki.save("orig\nremote change")

	if r := mustPush(t, dir, wiki, true); r[0].Action != Conflict {
		t.Fatalf("action = %s", r[0].Action)
	}
	if len(wiki.edits) != 0 {
		t.Error("a refused push must not edit")
	}
}

// The wiki moves between our conflict check and the edit landing: the edit is
// rejected server-side via baserevid, and nothing local moves.
func TestPushServerSideConflictLeavesBaseline(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	wiki.moveOnEdit = true

	if r := mustPush(t, dir, wiki, true); r[0].Action != PushError {
		t.Fatalf("action = %s, want error", r[0].Action)
	}
	if baseline, _, _ := store.ReadBaseline(dir, 1); baseline != "orig" {
		t.Errorf("baseline moved despite a rejected edit: %q", baseline)
	}
	if m, _ := store.LoadManifest(dir); m.Entries["A"].RevID != 5 {
		t.Errorf("manifest revid moved: %d", m.Entries["A"].RevID)
	}
}

func TestPushEditErrorIsReported(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	wiki.editErr = errors.New("boom")

	if r := mustPush(t, dir, wiki, true); r[0].Action != PushError {
		t.Fatalf("action = %s", r[0].Action)
	}
	if baseline, _, _ := store.ReadBaseline(dir, 1); baseline != "orig" {
		t.Errorf("baseline moved despite failed edit: %q", baseline)
	}
}

func TestPushDeletedLocalFileIsErrorNotRemoteDelete(t *testing.T) {
	wiki := newFakeWiki("orig", 5)
	dir := setupPulled(t, wiki, "orig\nnew")
	if err := os.Remove(filepath.Join(dir, "A.wiki")); err != nil {
		t.Fatal(err)
	}

	if r := mustPush(t, dir, wiki, true); r[0].Action != PushError {
		t.Fatalf("action = %s (a deleted local file must not trigger a remote edit)", r[0].Action)
	}
	if len(wiki.edits) != 0 {
		t.Error("must not edit for a missing local file")
	}
}
