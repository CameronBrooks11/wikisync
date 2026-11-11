# Commands

```
wikisync pull   [--host H] [--path P] [--dir D] [--force]
                [--category C]... [--prefix P]... [--all-languages] [--max N]
                [PAGE ...]
wikisync status [--dir D]
wikisync diff   [--dir D] [PAGE ...]
wikisync push   [--host H] [--path P] [--dir D] [--apply] [--summary S] [PAGE ...]
wikisync --version
wikisync --help
```

Flags come before page titles: `wikisync pull --dir ./pages "Some page"`. A
flag after the first title is read as a title.

`--dir` is the working tree (default: the current directory). `pull` accepts
titles with spaces or underscores (`Water Pump` or `Water_Pump`). `diff` and
`push` match titles exactly as `status` lists them, and report a title that
isn't tracked.

## pull

Fetches pages and writes each one to a `.wiki` file, recording the revision it
came from and keeping an untouched copy for `diff` and `push` (see
[Working tree](working-tree.md)).

What it fetches:

| You give | It pulls |
|---|---|
| Page titles | Those pages |
| `--category C` | The pages directly in `Category:C` (subcategories aren't followed) |
| `--prefix P` | Every page whose title starts with `P`, e.g. `User:You/` |
| Nothing | Every page already tracked in `--dir`, again |

`--category` and `--prefix` can be repeated and combined with titles; each page
is pulled once.

| Flag | Default | Meaning |
|---|---|---|
| `--all-languages` | off | Also pull category and prefix members in other languages. By default only pages in the wiki's own language are kept, which skips machine translations like `Page/es`. Pages you name are always pulled. |
| `--max N` | 500 | Refuse a category or prefix selection of more than N pages (counted after translations are dropped), before downloading anything. |
| `--force` | off | Overwrite local files that have changes you haven't pushed. Without it, those pages are skipped and reported. |

Output: one `pulled "Title" -> file` line per page on stdout; skipped pages
(missing, unpushed local changes, a file name another page already uses) and
the number of translations left out go to stderr.

Pages are fetched 50 to a request. Very large batches that MediaWiki splits
across responses are followed until every page has arrived.

## status

Lists every tracked page with its state. Offline.

| State | Meaning |
|---|---|
| `unchanged` | The file matches what was pulled |
| `modified` | You've changed the file |
| `deleted` | The file is gone (`push` never deletes the wiki page) |
| `baseline-unknown` | Pulled by a very old wikisync; pull it again |

## diff

Shows a unified diff of each changed file against the copy saved when it was
pulled. Offline. With titles, only those pages; a title that isn't tracked is
reported on stderr.

## push

Without `--apply`, `push` is a preview: for each changed page it checks the
wiki's current revision and prints `would push "Title"`, or a conflict. Nothing
is written to the wiki.

With `--apply` it logs in with a [bot password](credentials.md) and publishes.
`--summary` is required; it becomes the edit summary on the wiki.

For each tracked page (or each page you name), `push`:

1. skips it if the file matches what you pulled;
2. refuses it as a **conflict** if the page's current revision on the wiki is
   not the one you pulled, or the page no longer exists;
3. otherwise edits the page, passing the revision you pulled so the wiki also
   rejects the edit if someone saves in between;
4. reads back the text the wiki stored (it trims trailing whitespace, for
   example) and writes that to your file and baseline, so `status` is clean
   afterwards.

`push` never creates or deletes pages, and a missing local file is an error,
never a delete.

Output: `pushed "Title" (now rev N)`, `would push "Title"`, or a `conflict`,
`error` or `skipped` line on stderr with the reason.

## Exit codes

| Code | When |
|---|---|
| 0 | Success. For `pull`, at least one page was written; pages that don't exist on the wiki are reported but don't fail the run. |
| 1 | Something needs attention: `pull` skipped a page with local changes or a clashing file name, wrote nothing, or exceeded `--max`; `push` hit a conflict or error, or was given a title that isn't tracked; or the wiki or a file couldn't be read. |
| 2 | Usage: an unknown command or flag; `pull` with nothing named and nothing tracked; a first `pull` without `--host`; a `--host` or `--path` different from the working tree's wiki; or `push --apply` without `--summary` or credentials. |

## Choosing the wiki

wikisync works with any MediaWiki site over HTTPS. `--host` is the host name
and `--path` is the directory that holds `api.php` (default `/w/`, the usual
MediaWiki layout).

A working tree belongs to one wiki:

- The first `pull` into an empty `--dir` needs `--host`:
  `wikisync pull --host wiki.example.org --dir ./pages "Main Page"`.
- After that, `pull` and `push` in that folder use the wiki recorded in
  `.wikisync/manifest.json`; you don't pass `--host` or `--path` again.
- Passing a different `--host` or `--path` there is refused. Use a separate
  `--dir` for each wiki.

wikisync identifies itself to
the wiki with a `User-Agent` of `wikisync/<version>` and this project's URL.
