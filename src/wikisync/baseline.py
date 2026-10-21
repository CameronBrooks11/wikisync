"""Immutable snapshots of pulled wikitext.

Each pulled page's text is saved under ``.wikisync/baselines/<pageid>.wiki`` at
pull time. Keeping the original text (not just its hash) is what lets ``diff``
show a real unified diff offline, and gives ``push`` a known base for conflict
detection. Keyed by page id so a wiki-side rename does not orphan the snapshot.
"""

from __future__ import annotations

from pathlib import Path

from wikisync.manifest import MANIFEST_DIR

BASELINES_SUBDIR = "baselines"


def baseline_path(root: Path, pageid: int) -> Path:
    return root / MANIFEST_DIR / BASELINES_SUBDIR / f"{pageid}.wiki"


def write_baseline(root: Path, pageid: int, text: str) -> Path:
    path = baseline_path(root, pageid)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def read_baseline(root: Path, pageid: int) -> str | None:
    """Return the snapshot text, or None if there is none (e.g. a pre-v2 pull)."""
    path = baseline_path(root, pageid)
    if not path.exists():
        return None
    return path.read_text(encoding="utf-8")
