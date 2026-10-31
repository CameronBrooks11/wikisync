# wikisync

Work with MediaWiki pages — like [Appropedia](https://www.appropedia.org) — on
your own machine: pull pages as local wikitext, edit them with the tools you
already use (editor, grep, LLMs, cross-page search, offline), review a diff, then
push the changes back under your account. Like git, but for wikis.

> **Status: early.** `pull`, `status`, `diff` and `push` (preview and `--apply`)
> work and have been run against Appropedia. The `push --apply` round-trip also
> runs against a real MediaWiki 1.45 in CI on every change.

## Why

Editing wiki pages through a browser textarea is fine for one small change. It is
not fine for maintaining a set of pages: no real editor, no offline work, no
cross-page refactor, no LLM assistance, no local diff before you publish.
`wikisync` pulls the wikitext down so your normal toolchain applies, then pushes
it back through the MediaWiki API.

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

# Publish, authenticated as your account (requires a bot password + a summary).
export WIKISYNC_USERNAME_APPROPEDIA='You@botname'
export WIKISYNC_PASSWORD_APPROPEDIA='…'   # a MediaWiki bot password, never your login
wikisync push --dir ./pages --apply --summary "Fix typos"
```

`status` and `diff` are local only — they compare each file against the revision
recorded at pull time and never contact the wiki. `pull` keeps metadata in
`.wikisync/manifest.json` and an immutable snapshot of each page's pulled text in
`.wikisync/baselines/<pageid>.wiki`, which is what `diff` compares against. Pass
`--force` to `pull` to overwrite local files that have unpulled changes.

Each page is a `.wiki` file laid out like the wiki itself. Namespaces become
folders and subpages become subfolders, and spaces become underscores:

```
Welcome_to_Appropedia.wiki             Welcome to Appropedia
User/CameronBrooks11/wikisync.wiki     User:CameronBrooks11/wikisync
User_talk/Example.wiki                 User talk:Example
```

Characters that aren't safe in filenames are percent-encoded (`?` becomes
`%3F`). A title that can't be laid out safely as folders, such as one containing
`..` or an empty segment, becomes a single file with its slashes encoded, so
nothing is ever written outside the folder you pulled into. A page you pulled
before keeps the path it already has. If two pages would map to the same file
(for example `User:foo` and a main-namespace page `User/foo`), the second is
refused rather than overwriting the first.

`push` is preview-by-default and safe: it only writes with `--apply`, refuses
pages whose wiki revision moved past your baseline (re-pull to resolve), skips
unchanged pages, never deletes or creates remote pages (it only edits pages that
already exist), and advances a page's baseline only after a successful edit. The
wiki may save a slightly different form of what you sent (it trims trailing
whitespace, for example), so after a push your local file is rewritten to match
what the wiki actually stored.

Authentication uses a scoped MediaWiki bot password, never your account login.
Credentials are read per wiki: for `www.appropedia.org`, wikisync looks for
`WIKISYNC_USERNAME_APPROPEDIA` and `WIKISYNC_PASSWORD_APPROPEDIA` (the label
before the top-level domain, uppercased), so several wikis can be configured at
once. If that pair isn't set, it falls back to `WIKISYNC_USERNAME` and
`WIKISYNC_PASSWORD`. A pair is never mixed across the two. For hosts under a
two-part suffix such as `.co.uk`, use the generic names.

## Development

Requires Go and [`just`](https://github.com/casey/just).

```sh
just setup      # go mod download + install golangci-lint
just check      # gofmt-check + golangci-lint + go vet (what CI runs)
just test       # go test -race ./...
just build      # build ./bin/wikisync
```

### Testing

There are three layers:

| Command | What it exercises | Needs |
|---|---|---|
| `just test` | All logic, against a fake wiki that behaves like MediaWiki | nothing |
| `just test-integration` | The real push path against a throwaway MediaWiki 1.45 in Docker | Docker |
| `just test-live` | The same round-trip against a real wiki you choose | a test page + bot password |

CI runs the first two on every pull request. `test-integration` creates its own
admin, bot password and fixture page inside the container, so it needs no
secrets and never touches a real wiki.

`test-live` writes to, then restores, a **designated test page**. Never point it
at a real content page:

```sh
export WIKISYNC_TEST_PAGE='User:You/wikisync-test'
export WIKISYNC_USERNAME_APPROPEDIA='You@botname' WIKISYNC_PASSWORD_APPROPEDIA='…'
just test-live
```

Both integration recipes fail, rather than skip, when their settings are
missing, so they can't report success for a run that didn't happen.

## License

MIT — see [LICENSE](LICENSE).
