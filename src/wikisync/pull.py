"""``wikisync pull`` — fetch pages to local wikitext files and record a baseline."""

from __future__ import annotations

import re
from collections.abc import Callable, Iterable
from dataclasses import dataclass, field
from pathlib import Path

from wikisync.baseline import write_baseline
from wikisync.client import FetchedPage
from wikisync.manifest import Entry, Manifest, content_hash

# Characters unsafe in filenames across common filesystems (including '%' itself,
# so the encoding below stays reversible).
_UNSAFE = re.compile(r'[<>:"/\\|?*%\x00-\x1f]')


def title_to_filename(title: str) -> str:
    """Map a wiki title to a collision-safe, reversible ``.wiki`` filename.

    Spaces become underscores (MediaWiki treats the two as the same title), and
    every other unsafe character is percent-encoded. The mapping is injective for
    distinct titles, so ``Foo/Bar`` and ``Foo-Bar`` no longer collide.
    """
    name = title.replace(" ", "_")
    name = _UNSAFE.sub(lambda m: f"%{ord(m.group()):02X}", name)
    return f"{name}.wiki"


@dataclass
class PullReport:
    written: list[tuple[str, str]] = field(default_factory=list)  # (title, filename)
    missing: list[str] = field(default_factory=list)
    skipped_dirty: list[str] = field(default_factory=list)  # local edits would be lost
    collisions: list[tuple[str, str, str]] = field(default_factory=list)  # (title, file, owner)


def _filename_owner(manifest: Manifest, filename: str, title: str) -> str | None:
    """Return a *different* title that already maps to ``filename``, if any."""
    for other_title, entry in manifest.entries.items():
        if entry.file == filename and other_title != title:
            return other_title
    return None


def _is_clean(dest: Path, entry: Entry | None) -> bool:
    """True if the on-disk file matches its recorded baseline (safe to overwrite)."""
    return (
        entry is not None
        and bool(entry.sha256)
        and (content_hash(dest.read_text(encoding="utf-8")) == entry.sha256)
    )


def pull(
    titles: Iterable[str],
    fetch: Callable[[str], FetchedPage],
    root: Path,
    host: str,
    path: str,
    force: bool = False,
) -> PullReport:
    """Fetch each title, write its wikitext to ``root``, and update the manifest.

    Refuses to overwrite a local file that has unpulled changes (or is untracked)
    unless ``force`` is set; a clean tracked file is refreshed. ``fetch`` is
    injected so this stays testable without a network.
    """
    manifest = Manifest.load(root)
    report = PullReport()
    for title in titles:
        page = fetch(title)
        if not page.exists:
            report.missing.append(title)
            continue

        filename = title_to_filename(page.title)
        dest = root / filename

        owner = _filename_owner(manifest, filename, page.title)
        if owner is not None:
            report.collisions.append((page.title, filename, owner))
            continue

        if dest.exists() and not force and not _is_clean(dest, manifest.entries.get(page.title)):
            report.skipped_dirty.append(page.title)
            continue

        dest.write_text(page.text, encoding="utf-8")
        write_baseline(root, page.pageid, page.text)
        manifest.entries[page.title] = Entry(
            title=page.title,
            file=filename,
            revid=page.revid,
            host=host,
            path=path,
            sha256=content_hash(page.text),
            pageid=page.pageid,
        )
        report.written.append((page.title, filename))
    manifest.save(root)
    return report
