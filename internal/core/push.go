package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// PushAction is the outcome for one page.
type PushAction string

const (
	Pushed         PushAction = "pushed"
	WouldPush      PushAction = "would-push"
	SkipUnchanged  PushAction = "unchanged"
	Conflict       PushAction = "conflict"
	PushError      PushAction = "error"
	NoBaselinePush PushAction = "baseline-unavailable"
)

// PushResult is one page's push outcome.
type PushResult struct {
	Title    string
	Action   PushAction
	Detail   string
	NewRevID int64
}

// Remote is the wiki side of a push, injected so Push is testable without a
// network. CurrentRevID reports the live revision (for conflict detection); Edit
// publishes new wikitext with baseRevID as the server-side conflict base.
type Remote interface {
	CurrentRevID(title string) (revid int64, exists bool, err error)
	Edit(title, text, summary string, baseRevID int64) (newRevID int64, err error)
}

// Push compares each locally-modified page to its baseline and (when apply is
// true) publishes it. It refuses to push when the remote has moved past the
// recorded baseline revision (conflict), skips unchanged pages, never deletes
// remote pages, and updates the baseline only after a confirmed edit.
//
// With apply false it previews: every page that would be pushed is reported as
// would-push, and the wiki is only read, never written.
func Push(root string, titles []string, remote Remote, apply bool, summary string) ([]PushResult, error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return nil, err
	}

	selected := m.Entries
	if titles != nil {
		selected = map[string]store.Entry{}
		for _, t := range titles {
			if entry, ok := m.Entries[t]; ok {
				selected[t] = entry
			}
		}
	}

	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	results := make([]PushResult, 0, len(keys))
	changed := false
	for _, title := range keys {
		entry := selected[title]

		if entry.PageID == 0 {
			results = append(results, PushResult{Title: title, Action: NoBaselinePush, Detail: "no baseline; re-pull"})
			continue
		}
		baseline, ok, err := store.ReadBaseline(root, entry.PageID)
		if err != nil {
			return nil, err
		}
		if !ok {
			results = append(results, PushResult{Title: title, Action: NoBaselinePush, Detail: "no baseline snapshot; re-pull"})
			continue
		}

		data, err := os.ReadFile(filepath.Join(root, entry.File))
		if err != nil {
			// A missing/unreadable local file is never translated into a remote delete.
			results = append(results, PushResult{Title: title, Action: PushError, Detail: "local file unreadable: " + err.Error()})
			continue
		}
		local := string(data)
		if local == baseline {
			results = append(results, PushResult{Title: title, Action: SkipUnchanged})
			continue
		}

		remoteRev, exists, err := remote.CurrentRevID(title)
		if err != nil {
			results = append(results, PushResult{Title: title, Action: PushError, Detail: err.Error()})
			continue
		}
		if !exists {
			results = append(results, PushResult{Title: title, Action: Conflict, Detail: "page no longer exists on the wiki"})
			continue
		}
		if remoteRev != entry.RevID {
			results = append(results, PushResult{
				Title:  title,
				Action: Conflict,
				Detail: fmt.Sprintf("remote moved to rev %d (baseline rev %d); re-pull", remoteRev, entry.RevID),
			})
			continue
		}

		if !apply {
			results = append(results, PushResult{Title: title, Action: WouldPush})
			continue
		}

		newRev, err := remote.Edit(title, local, summary, entry.RevID)
		if err != nil {
			results = append(results, PushResult{Title: title, Action: PushError, Detail: err.Error()})
			continue
		}
		if err := store.WriteBaseline(root, entry.PageID, local); err != nil {
			return nil, err
		}
		entry.RevID = newRev
		entry.SHA256 = store.ContentHash(local)
		m.Entries[title] = entry
		changed = true
		results = append(results, PushResult{Title: title, Action: Pushed, NewRevID: newRev})
	}

	if changed {
		if err := m.Save(root); err != nil {
			return nil, err
		}
	}
	return results, nil
}
