// Command wikisync works with MediaWiki/Appropedia pages locally: pull pages as
// wikitext, see what changed, and (eventually) push edits back.
//
// Flags precede positional arguments, e.g. `wikisync pull --dir ./pages "Title"`.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/CameronBrooks11/wikisync/internal/core"
	"github.com/CameronBrooks11/wikisync/internal/mw"
)

const (
	version     = "0.1.0"
	defaultHost = "www.appropedia.org"
	defaultPath = "/w/"
)

func userAgent() string {
	return "wikisync/" + version + " (+https://github.com/CameronBrooks11/wikisync)"
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}
	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		return 0
	case "-v", "--version", "version":
		fmt.Println("wikisync " + version)
		return 0
	case "pull":
		return cmdPull(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "diff":
		return cmdDiff(args[1:])
	case "push":
		fmt.Fprintln(os.Stderr, "wikisync push: not yet implemented")
		return 2
	default:
		fmt.Fprintf(os.Stderr, "wikisync: unknown command %q (try 'wikisync --help')\n", args[0])
		return 2
	}
}

func printUsage() {
	fmt.Print(`wikisync — work with MediaWiki/Appropedia pages locally

Usage:
  wikisync pull   [--host H] [--path P] [--dir D] [--force] PAGE [PAGE ...]
  wikisync status [--dir D]
  wikisync diff   [--dir D] [PAGE ...]
  wikisync push   (not yet implemented)

Flags precede page titles, e.g.:
  wikisync pull --dir ./pages "Welcome to Appropedia"
`)
}

func cmdPull(args []string) int {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	host := fs.String("host", defaultHost, "wiki host")
	path := fs.String("path", defaultPath, "API script path")
	dir := fs.String("dir", ".", "directory to write files into")
	force := fs.Bool("force", false, "overwrite local files even if they have unpulled changes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	titles := fs.Args()
	if len(titles) == 0 {
		fmt.Fprintln(os.Stderr, "wikisync pull: need at least one page title")
		return 2
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "wikisync pull: %v\n", err)
		return 1
	}

	client := &http.Client{Timeout: 30 * time.Second}
	apiURL := mw.APIURL("https", *host, *path)
	fetch := func(title string) (mw.FetchedPage, error) {
		return mw.FetchPage(client, apiURL, userAgent(), title)
	}

	rep, err := core.Pull(titles, fetch, *dir, *host, *path, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wikisync pull: could not reach %s: %v\n", *host, err)
		return 1
	}
	for _, w := range rep.Written {
		fmt.Printf("pulled %q -> %s\n", w.Title, w.File)
	}
	for _, t := range rep.Missing {
		fmt.Fprintf(os.Stderr, "skipped %q: page does not exist\n", t)
	}
	for _, t := range rep.SkippedDirty {
		fmt.Fprintf(os.Stderr, "skipped %q: local file has unpulled changes (use --force to overwrite)\n", t)
	}
	for _, c := range rep.Collisions {
		fmt.Fprintf(os.Stderr, "skipped %q: filename %q is already used by %q\n", c.Title, c.File, c.Owner)
	}
	if len(rep.SkippedDirty) > 0 || len(rep.Collisions) > 0 || len(rep.Written) == 0 {
		return 1
	}
	return 0
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	dir := fs.String("dir", ".", "directory holding the pulled files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	items, err := core.Status(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wikisync status: %v\n", err)
		return 1
	}
	if len(items) == 0 {
		fmt.Println("nothing pulled yet; run 'wikisync pull <page>' first")
		return 0
	}
	for _, it := range items {
		fmt.Printf("%-16s %s (%s)\n", string(it.State), it.Title, it.File)
	}
	return 0
}

func cmdDiff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	dir := fs.String("dir", ".", "directory holding the pulled files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var selected []string
	if titles := fs.Args(); len(titles) > 0 {
		selected = titles
	}
	results, err := core.Diff(*dir, selected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wikisync diff: %v\n", err)
		return 1
	}

	found := map[string]bool{}
	for _, d := range results {
		found[d.Title] = true
	}
	for _, t := range selected {
		if !found[t] {
			fmt.Fprintf(os.Stderr, "diff: %q is not tracked (run 'wikisync pull' first)\n", t)
		}
	}

	printed := false
	for _, d := range results {
		switch d.State {
		case core.Changed:
			fmt.Print(d.Text)
			printed = true
		case core.DiffDeleted:
			fmt.Fprintf(os.Stderr, "%s: local file deleted\n", d.Title)
		case core.NoBaseline:
			fmt.Fprintf(os.Stderr, "%s: no baseline snapshot; re-pull to enable diff\n", d.Title)
		}
	}
	if !printed {
		fmt.Println("no local changes")
	}
	return 0
}
