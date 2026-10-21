# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project

`wikisync` is a CLI for working with MediaWiki pages (e.g. Appropedia) locally:
pull one or many pages as local wikitext files, edit them with any tools (IDE,
grep, LLMs, cross-page analysis, offline), review a diff, then push the edits
back under your own authenticated account. Git is the analogy, not the
transport — the transport is the MediaWiki API via `mwclient`.

Status: `pull` (fetch pages + record a baseline) and `status` (local diff
against that baseline, no network) are implemented; `push` is still a stub.

## Stack

- **Language** — Python ≥ 3.12, managed with `uv`.
- **MediaWiki client** — `mwclient` (lightweight MediaWiki API client).
- **Tooling** — `ruff` (format + lint), `pyright` (types), `pytest` (tests).

## Layout

```
src/wikisync/     package code (cli.py is the entrypoint)
tests/            pytest tests, mirroring src
```

## Commands

```sh
just setup        # uv sync — install deps + create the venv
just fmt          # format (ruff format + ruff check --fix)
just check        # fmt-check + lint + typecheck (CI-equivalent)
just test         # pytest
just run -- …     # run the CLI (e.g. just run -- --version)
```

## Conventions

- Format before committing; `just check` is the gate CI runs.
- Tests mirror source structure.
- Conventional Commits.

## Constraints

- **Pushing edits writes to a live, public wiki under a real user.** Treat every
  write path as ask-first: it must be explicit, dry-run-able, and never run by
  default. Reads are unauthenticated and safe.
- **Auth is a credential.** When push lands, authenticate via a MediaWiki bot
  password or OAuth pulled from the environment or a secret manager — never
  hardcode or commit credentials, and never log them.
- Identify the client to the API with a descriptive, honest `User-Agent`; do not
  put a personal email in requests unless the maintainer chooses to.
- Do not add dependencies without justification.
- Do not add AI attribution to commits or PR descriptions — no co-author
  trailers, session links, or "generated with" footers.
- Do not let a check report success when it could not run. A check that does not
  apply may pass, but must name itself as skipped; one that could not complete
  must not exit 0.
