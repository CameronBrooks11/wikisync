// Command wikisync works with MediaWiki/Appropedia pages locally: pull pages as
// wikitext, see what changed, and push edits back.
//
// Flags precede positional arguments, e.g. `wikisync pull --dir ./pages "Title"`.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/CameronBrooks11/wikisync/internal/core"
	"github.com/CameronBrooks11/wikisync/internal/creds"
	"github.com/CameronBrooks11/wikisync/internal/mw"
)

const (
	version     = "0.1.1"
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
		return cmdPush(args[1:])
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
  wikisync push   [--host H] [--path P] [--dir D] [--apply] [--summary S] [PAGE ...]

Flags precede page titles, e.g.:
  wikisync pull --dir ./pages "Welcome to Appropedia"

push previews by default; --apply publishes and needs a MediaWiki bot password
plus --summary. Credentials are read from WIKISYNC_USERNAME_<WIKI> and
WIKISYNC_PASSWORD_<WIKI>, where <WIKI> comes from the host
(www.appropedia.org -> APPROPEDIA), else from WIKISYNC_USERNAME and
WIKISYNC_PASSWORD.
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

	client := mw.NewClient(*host, *path, userAgent())
	rep, err := core.Pull(titles, client.FetchPage, *dir, *host, *path, *force)
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

func cmdPush(args []string) int {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	host := fs.String("host", defaultHost, "wiki host")
	path := fs.String("path", defaultPath, "API script path")
	dir := fs.String("dir", ".", "directory holding the pulled files")
	apply := fs.Bool("apply", false, "actually publish edits (default: preview only)")
	summary := fs.String("summary", "", "edit summary (required with --apply)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var selected []string
	if titles := fs.Args(); len(titles) > 0 {
		selected = titles
	}

	client := mw.NewClient(*host, *path, userAgent())

	if *apply {
		if *summary == "" {
			fmt.Fprintln(os.Stderr, "wikisync push --apply: --summary is required")
			return 2
		}
		c, ok := creds.Lookup(*host, os.Getenv)
		if !ok {
			fmt.Fprintf(os.Stderr, "wikisync push --apply: set %s (a MediaWiki bot password)\n", creds.Hint(*host))
			return 2
		}
		fmt.Fprintf(os.Stderr, "logging in as %s (from %s)\n", c.Username, c.Source)
		if err := client.Login(c.Username, c.Password); err != nil {
			fmt.Fprintf(os.Stderr, "wikisync push: login failed: %v\n", err)
			return 1
		}
	}

	results, err := core.Push(*dir, selected, client, *apply, *summary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wikisync push: %v\n", err)
		return 1
	}

	exit := 0
	actionable := false
	for _, r := range results {
		switch r.Action {
		case core.Pushed:
			fmt.Printf("pushed %q (now rev %d)\n", r.Title, r.NewRevID)
			actionable = true
		case core.WouldPush:
			fmt.Printf("would push %q\n", r.Title)
			actionable = true
		case core.SkipUnchanged:
			// quiet: nothing to do
		case core.Conflict:
			fmt.Fprintf(os.Stderr, "conflict %q: %s\n", r.Title, r.Detail)
			exit = 1
		case core.PushError:
			fmt.Fprintf(os.Stderr, "error %q: %s\n", r.Title, r.Detail)
			exit = 1
		case core.NoBaselinePush:
			fmt.Fprintf(os.Stderr, "skipped %q: %s\n", r.Title, r.Detail)
			exit = 1
		}
	}
	if !actionable && exit == 0 {
		if *apply {
			fmt.Println("nothing to push")
		} else {
			fmt.Println("no local changes to push")
		}
	}
	return exit
}
