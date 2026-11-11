# wikisync

Pull pages from any MediaWiki wiki into local `.wiki` files, edit them with the
tools you already use (an editor, `grep`, scripts, an LLM, offline), review the
diff, then push the changes back under your own account. Like git, for a wiki.

**Documentation: <https://cameronbrooks11.github.io/wikisync/>**

## Install

Download a binary for Linux, macOS or Windows from the
[releases](https://github.com/CameronBrooks11/wikisync/releases), or build with
Go 1.23+:

```sh
go install github.com/CameronBrooks11/wikisync@latest
```

## Quick start

```sh
# The first pull names the wiki; the folder remembers it from then on
wikisync pull --host wiki.example.org --dir ./pages "Main Page"
wikisync pull --dir ./pages --category "Recipes"

# Edit the .wiki files, then see what changed (offline)
wikisync status --dir ./pages
wikisync diff --dir ./pages

# Preview the push (publishes nothing), then publish with a bot password
wikisync push --dir ./pages
export WIKISYNC_USERNAME_EXAMPLE='You@wikisync' WIKISYNC_PASSWORD_EXAMPLE='…'
wikisync push --dir ./pages --apply --summary "Fix typos" "Main Page"
```

`push` previews by default, refuses pages someone else has edited since you
pulled them, never creates or deletes pages, and logs in with a scoped
MediaWiki [bot password](docs/credentials.md), never your account password.

## Status

The `push --apply` round trip runs against a real MediaWiki 1.45 in CI on every
change, and all four commands have been used against
[Appropedia](https://www.appropedia.org). wikisync is 0.x: the command line may
still change between minor versions; see the [changelog](CHANGELOG.md).

## Docs

- [Getting started](docs/getting-started.md)
- [Commands](docs/commands.md): every flag, output and exit code
- [Credentials](docs/credentials.md): setting up a bot password
- [Working tree](docs/working-tree.md): file layout, `.wikisync/`, conflicts
- [Development](docs/development.md): building, testing, releasing

[Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) ·
MIT — see [LICENSE](LICENSE).
