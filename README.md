# wikisync

Work with MediaWiki pages — like [Appropedia](https://www.appropedia.org) — on
your own machine: pull pages as local wikitext, edit them with the tools you
already use (IDE, grep, LLMs, cross-page search, offline), review a diff, then
push the changes back under your account. Like git, but for wikis.

> **Status: early.** `pull`, `status`, and `diff` work; `push` is not implemented
> yet.

## Why

Editing wiki pages through a browser textarea is fine for one small change. It
is not fine for maintaining a set of pages: no real editor, no offline work, no
cross-page refactor, no LLM assistance, no local diff before you publish.
`wikisync` pulls the wikitext down so your normal toolchain applies, then pushes
it back through the MediaWiki API.

## Install

Requires [`uv`](https://docs.astral.sh/uv/).

```sh
git clone https://github.com/CameronBrooks11/wikisync
cd wikisync
just setup      # or: uv sync
```

## Usage

```sh
# Fetch pages to local .wiki files (default wiki: appropedia.org)
uv run wikisync pull "Welcome to Appropedia" --dir ./pages

# Edit the .wiki files with any tools, then see what changed since the pull
uv run wikisync status --dir ./pages
# -> modified  Welcome to Appropedia (Welcome_to_Appropedia.wiki)

# Show the actual unified diff of your local edits vs. what you pulled
uv run wikisync diff --dir ./pages
```

`status` and `diff` are local only — they compare each file against the revision
recorded at pull time and never contact the wiki. `pull` keeps the metadata in
`.wikisync/manifest.json` and an immutable snapshot of each page's pulled text in
`.wikisync/baselines/`, which is what `diff` compares against.

Not implemented yet:

```sh
wikisync push                   # publish edits as your authenticated account
```

## Development

```sh
just check      # fmt-check + lint + typecheck (what CI runs)
just test       # pytest
```

## License

MIT — see [LICENSE](LICENSE).
