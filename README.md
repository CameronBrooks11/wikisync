# wikisync

Work with MediaWiki pages — like [Appropedia](https://www.appropedia.org) — on
your own machine: pull pages as local wikitext, edit them with the tools you
already use (editor, grep, LLMs, cross-page search, offline), review a diff, then
push the changes back under your account. Like git, but for wikis.

> **Status: early.** `pull`, `status`, `diff` and `push` work. `push` previews by
> default; `--apply` publishes to a live wiki and needs a bot password.

## Why

Editing wiki pages through a browser textarea is fine for one small change. It is
not fine for maintaining a set of pages: no real editor, no offline work, no
cross-page refactor, no LLM assistance, no local diff before you publish.
`wikisync` pulls the wikitext down so your normal toolchain applies, then (soon)
pushes it back through the MediaWiki API.

## Install

Download a binary from the [releases](https://github.com/CameronBrooks11/wikisync/releases),
or build from source with Go 1.23+:

```sh
go install github.com/CameronBrooks11/wikisync@latest
```

## Usage

Flags come before page titles.

```sh
# Fetch pages to local .wiki files (default wiki: appropedia.org)
wikisync pull --dir ./pages "Welcome to Appropedia"

# Edit the .wiki files with any tools, then classify what changed since the pull
wikisync status --dir ./pages
# -> modified  Welcome to Appropedia (Welcome_to_Appropedia.wiki)

# Show the actual unified diff of your local edits vs. what you pulled
wikisync diff --dir ./pages

# Preview what would be published (read-only; flags conflicts if the wiki moved)
wikisync push --dir ./pages

# Publish, authenticated as your account (requires a bot password + a summary)
export WIKISYNC_USERNAME='You@botname'
export WIKISYNC_PASSWORD='…'   # a MediaWiki bot password, never your login
wikisync push --dir ./pages --apply --summary "Fix typos"
```

`status` and `diff` are local only — they compare each file against the revision
recorded at pull time and never contact the wiki. `pull` keeps metadata in
`.wikisync/manifest.json` and an immutable snapshot of each page's pulled text in
`.wikisync/baselines/<pageid>.wiki`, which is what `diff` compares against. Pass
`--force` to `pull` to overwrite local files that have unpulled changes.

`push` is preview-by-default and safe: it only writes with `--apply`, refuses
pages whose wiki revision moved past your baseline (re-pull to resolve), skips
unchanged pages, never deletes remote pages, and advances a page's baseline only
after a successful edit. Authentication reads `WIKISYNC_USERNAME` /
`WIKISYNC_PASSWORD` from the environment — use a scoped MediaWiki bot password,
not your account login.

## Development

Requires Go and [`just`](https://github.com/casey/just).

```sh
just setup      # go mod download + install golangci-lint
just check      # gofmt-check + golangci-lint + go vet (what CI runs)
just test       # go test -race ./...
just build      # build ./bin/wikisync
```

A live push round-trip is an opt-in test, excluded from `just test` and CI. It
writes to, then restores, a **designated test page** (never a real content page):

```sh
export WIKISYNC_TEST_PAGE='User:You/sandbox'
export WIKISYNC_USERNAME='You@botname' WIKISYNC_PASSWORD='…'
just test-integration
```

## License

MIT — see [LICENSE](LICENSE).
