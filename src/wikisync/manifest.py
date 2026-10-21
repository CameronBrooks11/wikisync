"""Baseline manifest.

Records which pages were pulled and the revision each came from, so that a later
``status`` can show local edits and ``push`` knows what it is updating. Stored as
``.wikisync/manifest.json`` at the root of the working directory.
"""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from pathlib import Path

MANIFEST_DIR = ".wikisync"
MANIFEST_FILE = "manifest.json"


@dataclass
class Entry:
    """One pulled page: where it lives locally and the revision it tracks."""

    title: str
    file: str
    revid: int
    host: str
    path: str


@dataclass
class Manifest:
    version: int = 1
    entries: dict[str, Entry] = field(default_factory=dict)  # keyed by page title

    @classmethod
    def load(cls, root: Path) -> Manifest:
        path = root / MANIFEST_DIR / MANIFEST_FILE
        if not path.exists():
            return cls()
        data = json.loads(path.read_text(encoding="utf-8"))
        entries = {title: Entry(**entry) for title, entry in data.get("entries", {}).items()}
        return cls(version=int(data.get("version", 1)), entries=entries)

    def save(self, root: Path) -> Path:
        directory = root / MANIFEST_DIR
        directory.mkdir(parents=True, exist_ok=True)
        path = directory / MANIFEST_FILE
        payload = {
            "version": self.version,
            "entries": {title: asdict(entry) for title, entry in self.entries.items()},
        }
        path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        return path
