# Getting started

## Install

Download the archive for your platform from the
[releases page](https://github.com/CameronBrooks11/wikisync/releases)
(Linux, macOS and Windows, amd64 and arm64), check it against `checksums.txt`,
and put the `wikisync` binary on your `PATH`.

Or build it with Go 1.23 or later:

```sh
go install github.com/CameronBrooks11/wikisync@latest
```

Check it runs:

```sh
wikisync --version
```

## Pull a page

The first pull into a folder names the wiki with `--host`:

```sh
wikisync pull --host wiki.example.org --dir ./pages "Main Page"
# pulled "Main Page" -> Main_Page.wiki
```

If the wiki's `api.php` isn't under `/w/`, also pass `--path` (see
[Commands](commands.md#choosing-the-wiki)). From then on the folder remembers
its wiki, so later commands in it need neither flag. Flags always come before
page titles.

To pull a set of pages, name a category or a title prefix:

```sh
wikisync pull --dir ./pages --category "Recipes"
wikisync pull --dir ./pages --prefix "User:You/"
```

`--category` takes only the pages directly in the category. Add `--depth 1` to
include its subcategories, `--depth 2` for theirs, and so on.

## Edit and review

Edit the `.wiki` files however you like. Then see what changed:

```sh
wikisync status --dir ./pages
# modified         Main Page (Main_Page.wiki)

wikisync diff --dir ./pages
```

Neither command contacts the wiki: they compare your files with the copy saved
when you pulled.

## Push

First preview. This checks each changed page against the wiki and publishes
nothing:

```sh
wikisync push --dir ./pages
# would push "Main Page"
```

To publish, you need a MediaWiki bot password (see [Credentials](credentials.md))
and an edit summary:

```sh
export WIKISYNC_USERNAME_EXAMPLE='You@wikisync'
export WIKISYNC_PASSWORD_EXAMPLE='…'
wikisync push --dir ./pages --apply --summary "Fix typos" "Main Page"
# logging in as You@wikisync (from WIKISYNC_USERNAME_EXAMPLE)
# pushed "Main Page" (now rev 1234567)
```

Name the pages you mean to publish, as above; with no titles, `push` takes
every changed page.

If someone edited a page on the wiki since you pulled it, `push` refuses it with
a conflict instead of overwriting their edit. Save your version, pull the page
again, and redo your change on top. See
[Working tree](working-tree.md#conflicts).

## Later

Pull every page you're tracking again, to pick up other people's edits:

```sh
wikisync pull --dir ./pages
```

Files you've changed and not pushed are left alone, and reported.
