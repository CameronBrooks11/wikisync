# wikisync

Work with MediaWiki pages — like [Appropedia](https://www.appropedia.org) — on
your own machine: pull pages as local wikitext, edit them with the tools you
already use (editor, grep, LLMs, cross-page search, offline), review a diff, then
push the changes back under your account. Like git, but for wikis.

> **Status: early.** `pull`, `status`, and `diff` work; `push` is not implemented
> yet.

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
```

`status` and `diff` are local only — they compare each file against the revision
recorded at pull time and never contact the wiki. `pull` keeps metadata in
`.wikisync/manifest.json` and an immutable snapshot of each page's pulled text in
`.wikisync/baselines/<pageid>.wiki`, which is what `diff` compares against. Pass
`--force` to `pull` to overwrite local files that have unpulled changes.

## Development

Requires Go and [`just`](https://github.com/casey/just).

```sh
just setup      # go mod download + install golangci-lint
just check      # gofmt-check + golangci-lint + go vet (what CI runs)
just test       # go test -race ./...
just build      # build ./bin/wikisync
```

## License

MIT — see [LICENSE](LICENSE).
