"""``wikisync pull`` — fetch pages to local wikitext files and record a baseline."""

from __future__ import annotations

import re
from collections.abc import Callable, Iterable
from dataclasses import dataclass, field
from pathlib import Path

from wikisync.client import FetchedPage
from wikisync.manifest import Entry, Manifest, content_hash

# Characters that are unsafe in filenames across common filesystems.
_UNSAFE = re.compile(r'[<>:"/\\|?*\x00-\x1f]')


def title_to_filename(title: str) -> str:
    """Map a wiki page title to a flat, filesystem-safe ``.wiki`` filename."""
    name = _UNSAFE.sub("-", title.replace(" ", "_"))
    return f"{name}.wiki"


@dataclass
class PullReport:
    written: list[tuple[str, str]] = field(default_factory=list)  # (title, filename)
    missing: list[str] = field(default_factory=list)


def pull(
    titles: Iterable[str],
    fetch: Callable[[str], FetchedPage],
    root: Path,
    host: str,
    path: str,
) -> PullReport:
    """Fetch each title, write its wikitext to ``root``, and update the manifest.

    ``fetch`` is injected so this stays testable without a network: the CLI passes
    a function backed by :mod:`wikisync.client`.
    """
    manifest = Manifest.load(root)
    report = PullReport()
    for title in titles:
        page = fetch(title)
        if not page.exists:
            report.missing.append(title)
            continue
        filename = title_to_filename(page.title)
        (root / filename).write_text(page.text, encoding="utf-8")
        manifest.entries[page.title] = Entry(
            title=page.title,
            file=filename,
            revid=page.revid,
            host=host,
            path=path,
            sha256=content_hash(page.text),
        )
        report.written.append((page.title, filename))
    manifest.save(root)
    return report
