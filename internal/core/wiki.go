package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/CameronBrooks11/wikisync/internal/store"
)

// ErrNoHost means a working tree has no pages yet and no --host was given.
var ErrNoHost = errors.New("--host is required for a working tree with no pages yet, e.g. --host wiki.example.org")

// WikiMismatchError reports a --host or --path that differs from the wiki a
// working tree already tracks.
type WikiMismatchError struct {
	Tracked string // the wiki(s) the tree tracks, e.g. "wiki.example.org/w/"
	Given   string // the wiki asked for
}

func (e *WikiMismatchError) Error() string {
	return fmt.Sprintf("this working tree tracks pages from %s, not %s; use a separate --dir for another wiki", e.Tracked, e.Given)
}

// Wiki is a wiki's address: its host and the directory holding api.php, e.g.
// {"wiki.example.org", "/w/"}.
type Wiki struct {
	Host string
	Path string
}

func (w Wiki) String() string { return w.Host + w.Path }

// ResolveWiki picks the wiki a command in root talks to. host and path are the
// --host and --path given ("" when not given). A tree with pages uses the wiki
// recorded in its manifest, and refuses a different one; an empty tree needs
// host, with path defaulting to defaultPath.
func ResolveWiki(root, host, path, defaultPath string) (Wiki, error) {
	m, err := store.LoadManifest(root)
	if err != nil {
		return Wiki{}, err
	}
	seen := map[Wiki]bool{}
	var tracked []Wiki
	for _, e := range m.Entries {
		w := Wiki{e.Host, e.Path}
		if w.Host != "" && !seen[w] {
			seen[w] = true
			tracked = append(tracked, w)
		}
	}
	sort.Slice(tracked, func(i, j int) bool { return tracked[i].String() < tracked[j].String() })

	if len(tracked) == 0 {
		if host == "" {
			return Wiki{}, ErrNoHost
		}
		if path == "" {
			path = defaultPath
		}
		return Wiki{host, path}, nil
	}

	names := make([]string, len(tracked))
	for i, w := range tracked {
		names[i] = w.String()
	}
	// Trees made before 0.3.0 can mix wikis; then --host must pick one.
	if host == "" && len(tracked) > 1 {
		return Wiki{}, fmt.Errorf("this working tree tracks pages from %s; pass --host to choose one", strings.Join(names, ", "))
	}
	for _, w := range tracked {
		if (host == "" || host == w.Host) && (path == "" || path == w.Path) {
			return w, nil
		}
	}
	given := host + path
	switch {
	case host == "":
		given = "--path " + path
	case path == "":
		given = host
	}
	return Wiki{}, &WikiMismatchError{Tracked: strings.Join(names, ", "), Given: given}
}
