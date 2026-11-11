# Working tree

The folder you pull into (`--dir`) is the working tree: one `.wiki` file per
page, plus a `.wikisync/` folder that records what you pulled.

```
pages/
├── Main_Page.wiki                    Main Page
├── Simple_living/
│   └── Rural_Architecture.wiki       Simple living/Rural Architecture
├── User/
│   └── You/
│       └── notes.wiki                User:You/notes
└── .wikisync/
    ├── manifest.json
    └── baselines/
        └── 146465.wiki
```

## File names

- Each namespace is a folder (`User/`, `User_talk/`, `Template/`); main-namespace
  pages sit at the top. The namespace comes from the wiki, so a colon that is
  part of a main-namespace title stays in the file name.
- Subpages (`Page/Sub`) become subfolders.
- Spaces become underscores, and characters that aren't safe in file names are
  percent-encoded (`?` becomes `%3F`, `:` in a title becomes `%3A`).
- A title that can't be laid out safely as folders, such as one with `..` or an
  empty segment, is saved as a single file with its slashes encoded. No title
  can make wikisync write outside the working tree.
- If two pages would map to the same file (`User:foo` and a main-namespace page
  `User/foo`), the second is refused rather than overwriting the first.
- A page keeps the path it was first pulled to, so upgrading wikisync never moves
  existing files.

Files are stored exactly as the wiki returns them. MediaWiki strips a page's
final newline, so pulled files usually have none; an editor set to add one will
make a file look changed. `push` handles that case (see below), but you can tell
your editor to leave `*.wiki` files alone.

## `.wikisync/`

`manifest.json` lists every tracked page:

```json
{
  "version": 2,
  "entries": {
    "User:You/notes": {
      "title": "User:You/notes",
      "file": "User/You/notes.wiki",
      "revid": 1292640,
      "host": "wiki.example.org",
      "path": "/w/",
      "sha256": "…",
      "pageid": 146465
    }
  }
}
```

`host` and `path` are the tree's wiki: later commands in the folder use them,
and refuse a different `--host`. `revid` is the revision you pulled or last
pushed, and `sha256` is the hash of that text, which is how `status` spots
changes without the network.

`baselines/<pageid>.wiki` holds that text itself. `diff` compares against it,
and `push` uses it to tell an edited file from an unedited one.

Don't edit `.wikisync/` by hand. If you keep the working tree in git, commit it:
it's what lets a clone carry on where you left off.

## Conflicts

`push` only edits a page if its current revision on the wiki is the one in your
manifest. If someone else has saved the page since, you get:

```
conflict "Some page": remote moved to rev 1300002 (baseline rev 1299990); re-pull
```

and nothing is written. The edit itself also carries the revision you pulled
(`baserevid`), so MediaWiki can detect a save made between the check and the
edit.

To resolve it:

1. Copy your version of the file somewhere outside the working tree.
2. `wikisync pull --dir ./pages --force "Some page"` to take the current text.
3. Re-apply your change on top, then preview and push again.

wikisync doesn't merge for you.

## What the wiki changes on save

MediaWiki may store a slightly different text from the one you sent; it trims
trailing whitespace, for example. After a successful edit, `push` reads back
what the wiki stored and writes that to your file and baseline, so `status`
shows the page as unchanged. If the wiki's saved form is identical to what's
already there, no revision is made and your file is reset to the stored text.
