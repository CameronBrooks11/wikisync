# wikisync

Work with MediaWiki pages — like [Appropedia](https://www.appropedia.org) — on
your own machine: pull pages as local wikitext, edit them with the tools you
already use (IDE, grep, LLMs, cross-page search, offline), review a diff, then
push the changes back under your account. Like git, but for wikis.

> **Status: early scaffold.** The CLI exposes its intended shape (`pull`,
> `status`, `push`) but the commands are not implemented yet.

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
just run -- --version
uv run wikisync --help
```

Planned workflow (not yet implemented):

```sh
wikisync pull <page|category>   # fetch pages to local wikitext files
wikisync status                 # show local edits vs. the fetched revisions
wikisync push                   # publish edits as your authenticated account
```

## Development

```sh
just check      # fmt-check + lint + typecheck (what CI runs)
just test       # pytest
```

## License

MIT — see [LICENSE](LICENSE).
