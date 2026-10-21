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

`push` logs in with a bot password from `WIKISYNC_USERNAME`/`WIKISYNC_PASSWORD`,
passes `baserevid` for server-side conflict detection, refuses pages whose remote
revision moved past the baseline, skips unchanged pages, never deletes remote
pages, and advances the baseline only after a confirmed edit. It has not yet been
exercised against a live wiki in an integration test — do that against a
controlled instance or a designated test page, never live Appropedia.

## Stack

- **Language** — Go 1.23+.
- **Dependencies** — standard library for the read client; `go-difflib` for
  unified diffs. No MediaWiki client library yet (the read path is plain
  HTTP+JSON); `push` will add one for auth/edit tokens.
- **Tooling** — `gofmt`, `golangci-lint`, `go vet`, `go test -race`.

## Layout

```
main.go              CLI: flag parsing and dispatch
internal/mw/         MediaWiki API read client
internal/store/      manifest + baseline snapshots (.wikisync/)
internal/core/       pull, status, diff
```

## Commands

```sh
just setup        # go mod download + install golangci-lint
just fmt          # gofmt -w
just check        # fmt-check + lint + vet (CI-equivalent)
just test         # go test -race ./...
just build        # build ./bin/wikisync
just run -- ...    # go run . ...
```

## Conventions

- CLI flags precede positional titles (`wikisync pull --dir D "Title"`).
- The `.wikisync/` on-disk format is language-agnostic and versioned
  (`store.CurrentVersion`); bump it and handle old manifests when it changes.
- Keep `status` and `diff` offline; only `pull`/`push` touch the network.
- Conventional Commits.

## Constraints

- **Pushing edits writes to a live, public wiki under a real user.** Treat every
  write path as ask-first: explicit, dry-run-able, never run by default. Reads
  are unauthenticated and safe.
- **Auth is a credential.** When `push` lands, read a bot password or OAuth token
  from the environment or a secret manager — never hardcode, commit, or log it.
- Identify the client to the API with an honest `User-Agent`; no personal email.
- Do not add dependencies without justification.
- Do not add AI attribution to commits or PR descriptions.
- Do not let a check report success when it could not run.
