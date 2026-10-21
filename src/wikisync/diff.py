"""``wikisync diff`` — unified diff of local files against the pulled baseline.

Local only: it compares each working file to its baseline snapshot and never
contacts the wiki.
"""

from __future__ import annotations

import difflib
from dataclasses import dataclass, field
from enum import StrEnum
from pathlib import Path

from wikisync.baseline import read_baseline
from wikisync.manifest import Manifest


class DiffState(StrEnum):
    CHANGED = "changed"
    UNCHANGED = "unchanged"
    DELETED = "deleted"
    NO_BASELINE = "baseline-unavailable"


@dataclass
class PageDiff:
    title: str
    state: DiffState
    lines: list[str] = field(default_factory=list)  # unified-diff lines when CHANGED


def diff(root: Path, titles: list[str] | None = None) -> list[PageDiff]:
    """Diff each tracked page (or the named ones) against its baseline snapshot."""
    manifest = Manifest.load(root)
    selected = manifest.entries if titles is None else {}
    if titles is not None:
        selected = {t: manifest.entries[t] for t in titles if t in manifest.entries}

    results: list[PageDiff] = []
    for title, entry in sorted(selected.items()):
        baseline = read_baseline(root, entry.pageid) if entry.pageid else None
        if baseline is None:
            results.append(PageDiff(title, DiffState.NO_BASELINE))
            continue
        file_path = root / entry.file
        if not file_path.exists():
            results.append(PageDiff(title, DiffState.DELETED))
            continue
        local = file_path.read_text(encoding="utf-8")
        if local == baseline:
            results.append(PageDiff(title, DiffState.UNCHANGED))
            continue
        lines = list(
            difflib.unified_diff(
                baseline.splitlines(keepends=True),
                local.splitlines(keepends=True),
                fromfile=f"a/{entry.file} (rev {entry.revid})",
                tofile=f"b/{entry.file} (local)",
            )
        )
        results.append(PageDiff(title, DiffState.CHANGED, lines))
    return results
