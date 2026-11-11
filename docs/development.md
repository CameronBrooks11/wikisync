# Development

wikisync is a single Go binary with one dependency (`go-difflib`, for unified
diffs). The MediaWiki client uses only the standard library.

## Layout

```
main.go              CLI: flags and dispatch
internal/mw/         MediaWiki API client: pages, listings, login, edit
internal/core/       pull and page selection, status, diff, push, file paths
internal/store/      manifest and baseline snapshots (.wikisync/)
internal/creds/      per-wiki bot-password lookup from the environment
scripts/test-wiki.sh throwaway MediaWiki in Docker for integration tests
docs/                this documentation, published with GitHub Pages
```

## Build and check

You need Go 1.23 or later and [`just`](https://github.com/casey/just).

```sh
just setup     # download modules, install golangci-lint
just fmt       # gofmt -w
just check     # format check, golangci-lint, go vet (what CI runs)
just test      # go test -race ./...
just build     # ./bin/wikisync
```

## Tests

| Command | Exercises | Needs |
|---|---|---|
| `just test` | All logic, against a fake wiki that behaves like MediaWiki: trailing-whitespace trimming, null edits, stale-revision conflicts | nothing |
| `just test-integration` | The real push path against a throwaway MediaWiki 1.45 | Docker |
| `just test-live` | The same round trip against a real wiki | a designated test page and a bot password |

CI runs the first two on every pull request. `test-integration` creates its own
admin account, bot password and fixture page inside the container, so it needs
no secrets and never touches a real wiki.

`test-live` edits a page and then restores it. Point it only at a page set
aside for testing, never at real content:

```sh
export WIKISYNC_TEST_HOST='wiki.example.org' WIKISYNC_TEST_PAGE='User:You/wikisync-test'
export WIKISYNC_USERNAME_EXAMPLE='You@wikisync' WIKISYNC_PASSWORD_EXAMPLE='…'
just test-live
```

Both integration recipes fail, rather than skip, when their settings are
missing, so they can't report success for a run that didn't happen.

## Releases

Version numbers follow semver while wikisync is 0.x. To release:

1. Bump `version` in `main.go` and add the release to `CHANGELOG.md`, in one PR.
2. After it merges, tag the merge commit and push the tag:
   `git tag -a v0.3.0 -m "wikisync v0.3.0" && git push origin v0.3.0`.
3. The release workflow builds the binaries and checksums with GoReleaser.
   Download one and run `--version` to confirm.

## Docs

The site is built from `docs/` by Jekyll on GitHub Pages whenever `docs/`
changes on `main`. The pages are plain Markdown, so they also read correctly on
GitHub. Links between pages use the `.md` file name.
