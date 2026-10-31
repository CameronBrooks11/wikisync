# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project

`wikisync` is a CLI for working with MediaWiki pages (e.g. Appropedia) locally:
pull one or many pages as local wikitext files, edit them with any tools (editor,
grep, LLMs, cross-page analysis, offline), review a diff, then push the edits back
under your own authenticated account. Git is the analogy, not the transport — the
transport is the MediaWiki JSON API.

Status: `pull` (fetch pages, snapshot a baseline, record it in the manifest),
`status` (local change classification), `diff` (local unified diff vs the
baseline) and `push` (preview by default; `--apply` publishes) are implemented.
`status` and `diff` are offline; `pull` and `push` touch the network. Baselines
live in `.wikisync/baselines/<pageid>.wiki`; the manifest is schema v2.

`pull` selects pages through `core.PullSet`: explicit titles, `--category`,
`--prefix`, or (with none) every tracked page. Listed pages carry their page
language, so translations are dropped before the `--max` cap and before any text
is fetched; texts come in batches of 50 (`mw.FetchPages`). Per-page writing,
dirty-file and collision checks stay in `core.Pull`.

`push` logs in with a bot password, read per wiki from
`WIKISYNC_USERNAME_<WIKI>`/`WIKISYNC_PASSWORD_<WIKI>` (`<WIKI>` derived from the
host by `internal/creds`, e.g. `APPROPEDIA`), falling back to
`WIKISYNC_USERNAME`/`WIKISYNC_PASSWORD`,
passes `baserevid` for server-side conflict detection, refuses pages whose remote
revision moved past the baseline, skips unchanged pages, never deletes remote
pages, and advances the baseline to the text the wiki actually stored after a
confirmed edit.

Testing has three layers. `just test` runs the unit tests against a fake wiki
that mimics MediaWiki's save behaviour (trailing-whitespace trim, null edits with
no new revision, stale-`baserevid` conflicts). `just test-integration` runs the
round-trip test (build tag `integration`) against a throwaway MediaWiki 1.45 in
Docker via `scripts/test-wiki.sh`, which creates its own credentials; CI runs it
on every PR. `just test-live` runs the same test against a real wiki's
**designated test page** (`WIKISYNC_TEST_PAGE`) with real bot-password
credentials, and is never part of CI. Agent sessions may not be able to reach
the Docker socket; in that case CI is the runner for `test-integration`.

## Stack

- **Language** — Go 1.23+.
- **Dependencies** — standard library for the MediaWiki client (reads, login,
  edits: plain HTTP+JSON with a cookie jar); `go-difflib` for unified diffs.
- **Tooling** — `gofmt`, `golangci-lint`, `go vet`, `go test -race`.

## Layout

```
main.go              CLI: flag parsing and dispatch
internal/mw/         MediaWiki API client: pages, listings, login, edit
internal/store/      manifest + baseline snapshots (.wikisync/)
internal/core/       pull (and page selection), status, diff, push, paths
internal/creds/      per-wiki bot-password lookup from the environment
scripts/test-wiki.sh throwaway MediaWiki in Docker for test-integration
```

## Commands

```sh
just setup        # go mod download + install golangci-lint
just fmt          # gofmt -w
just check        # fmt-check + lint + vet (CI-equivalent)
just test         # go test -race ./...
just test-integration  # push round-trip vs throwaway MediaWiki in Docker
just test-live    # same vs a real wiki's designated test page (never CI)
just build        # build ./bin/wikisync
just run -- ...    # go run . ...
```

## Conventions

- CLI flags precede positional titles (`wikisync pull --dir D "Title"`).
- The `.wikisync/` on-disk format is language-agnostic and versioned
  (`store.CurrentVersion`); bump it and handle old manifests when it changes.
- Keep `status` and `diff` offline; only `pull`/`push` touch the network.
- Working-tree paths come from `core.PagePath` (namespace folder, subpage
  subfolders). Titles come from a remote wiki, so keep its segment check: no
  title may yield a path outside the working tree. Manifest `File` values are
  slash-separated; join them with `filepath.FromSlash`.
- Conventional Commits.

## Constraints

- **Pushing edits writes to a live, public wiki under a real user.** Treat every
  write path as ask-first: explicit, dry-run-able, never run by default. Reads
  are unauthenticated and safe.
- **Auth is a credential.** `push` reads a bot password from the environment
  (`internal/creds`) — never hardcode, commit, or log it.
- Identify the client to the API with an honest `User-Agent`; no personal email.
- Do not add dependencies without justification.
- Do not add AI attribution to commits or PR descriptions.
- Do not let a check report success when it could not run.
