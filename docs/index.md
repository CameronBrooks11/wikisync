# wikisync

wikisync copies pages from any MediaWiki wiki into local `.wiki` files, so you can
edit them with the tools you already use: an editor, `grep`, scripts, an LLM,
cross-page search, or work offline. When you're done it shows you exactly what
changed and pushes the edits back under your own account.

It works like a small, wiki-specific git:

| Command | Does | Network |
|---|---|---|
| `wikisync pull` | Fetch pages (by title, category or prefix) and record what you fetched | read |
| `wikisync status` | List which local files you changed | none |
| `wikisync diff` | Show your changes as a unified diff | none |
| `wikisync push` | Preview what would be published; with `--apply`, publish it | read, then write |

`push` previews by default, refuses to overwrite a page someone else changed
since you pulled it, never deletes or creates pages, and logs in with a scoped
[bot password](credentials.md) rather than your account password.

## Where to go next

- [Getting started](getting-started.md): install, pull a page, edit it, push it.
- [Commands](commands.md): every command and flag, its output and exit codes.
- [Credentials](credentials.md): set up a bot password for publishing.
- [Working tree](working-tree.md): how files are laid out, what `.wikisync/`
  holds, and how `push` avoids overwriting other people's edits.
- [Development](development.md): build, test and release wikisync itself.

Source, releases and issues are on
[GitHub](https://github.com/CameronBrooks11/wikisync). wikisync is MIT
licensed.
