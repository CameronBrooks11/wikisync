# Changelog

All notable changes to wikisync. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
semver while wikisync is 0.x. Full notes for each release are on the
[releases page](https://github.com/CameronBrooks11/wikisync/releases).

## [Unreleased]

### Fixed

- `push --apply` and `pull --watchlist` report the HTTP status when the wiki
  answers login or an edit with an error page (e.g. `503 Service
  Unavailable`), instead of `invalid character '<'`. An edit refused by the
  wiki now reads `mediawiki api error <code>: <info>`, like every other API
  error. (#45)

## [0.5.0] - 2026-11-12

### Added

- `pull --watchlist` pulls the pages on your watchlist. It logs in with the
  same bot password as `push --apply`, which needs the "View your watchlist"
  grant. Talk pages and watched titles with no page are left out; translations
  and `--max` are handled as for `--category`. (#22)

## [0.4.0] - 2026-11-12

### Added

- `pull --depth N` follows `--category` into subcategories, N levels down.
  Each category is listed once, so categories that contain each other don't
  loop, and `--max` counts the pages of the whole tree. (#21)

## [0.3.1] - 2026-11-11

### Fixed

- `push` reports a title that isn't tracked and exits 1. It used to ignore it
  and print "no local changes to push", so a mistyped title looked like a run
  with nothing to publish. (#24)

## [0.3.0] - 2026-11-11

### Changed

- **No default wiki.** wikisync no longer assumes Appropedia. The first `pull`
  into an empty folder needs `--host` (and `--path` if `api.php` isn't under
  `/w/`). After that, `pull` and `push` in the folder use the wiki recorded in
  its manifest, and refuse a different `--host` or `--path`.

  Existing working trees already record their wiki, so they keep working
  without `--host`. Only a first pull into a new folder changes:
  `wikisync pull --dir ./pages "Page"` becomes
  `wikisync pull --host www.appropedia.org --dir ./pages "Page"`.
- `just test-live` needs `WIKISYNC_TEST_HOST`; it no longer defaults to
  Appropedia.

### Added

- Documentation in `docs/`, published at
  <https://cameronbrooks11.github.io/wikisync/>.

## [0.2.1] - 2026-10-31

### Fixed

- Pulling many large pages no longer reports some of them as missing. When a
  batch of 50 pages exceeded MediaWiki's 8 MiB response limit, 0.2.0 skipped
  the pages left out of the response and still exited 0. (#19)

## [0.2.0] - 2026-10-31

### Added

- `pull --category` and `pull --prefix`, repeatable and combinable with titles.
  Subcategories aren't followed. (#17)
- A bare `pull` pulls every tracked page again. (#17)
- Category and prefix pulls keep only pages in the wiki's own language unless
  `--all-languages` is given; pages named explicitly are always pulled. (#17)
- `--max` (default 500) refuses an oversized selection before anything is
  downloaded. (#17)
- Pages are fetched 50 per request. (#17)

### Changed

- `push --apply` is no longer experimental: the round trip passed against
  Appropedia on 2026-10-30. (#12, #16)

## [0.1.2] - 2026-10-23

### Changed

- Pulled pages are laid out like the wiki: a folder per namespace and
  subfolders for subpages. Pages pulled by an earlier version keep their
  paths. (#14)

### Security

- Titles are checked before files are written: one that can't be laid out
  safely is saved as a single encoded file, and two pages mapping to the same
  file are refused rather than overwritten. (#14)

## [0.1.1] - 2026-10-22

### Fixed

- A null edit recorded revision 0 as the baseline, after which the page could
  never be pushed again. `push` now records the revision the wiki stored. (#9)
- After a push, the baseline and local file now hold the text the wiki stored,
  not the text sent. (#9)
- API errors show MediaWiki's own code and message. (#10)

### Added

- Per-wiki credentials: `WIKISYNC_USERNAME_<WIKI>` / `WIKISYNC_PASSWORD_<WIKI>`,
  falling back to the generic names. (#11)

## [0.1.0] - 2026-10-21

### Added

- First release, in Go: `pull`, `status`, `diff` and `push` (preview by
  default; `--apply` to publish, experimental in this release), with baseline
  snapshots for offline diffs and revision checks before every push.

[Unreleased]: https://github.com/CameronBrooks11/wikisync/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/CameronBrooks11/wikisync/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/CameronBrooks11/wikisync/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/CameronBrooks11/wikisync/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/CameronBrooks11/wikisync/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/CameronBrooks11/wikisync/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/CameronBrooks11/wikisync/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/CameronBrooks11/wikisync/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/CameronBrooks11/wikisync/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/CameronBrooks11/wikisync/releases/tag/v0.1.0
