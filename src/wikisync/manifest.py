"""Baseline manifest.

Records which pages were pulled, the revision each came from, and a hash of the
pulled text, so ``status``/``diff`` can show local edits and ``push`` knows what
it is updating. Stored as ``.wikisync/manifest.json`` at the root of the working
directory; the pulled text itself is snapshotted under ``.wikisync/baselines/``.
"""

from __future__ import annotations

import hashlib
import json
import os
from dataclasses import asdict, dataclass, field
from pathlib import Path

MANIFEST_DIR = ".wikisync"
MANIFEST_FILE = "manifest.json"
CURRENT_VERSION = 2  # v2 added Entry.pageid and baseline snapshots


def content_hash(text: str) -> str:
    """Stable content hash of wikitext, used to detect local edits since a pull."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


@dataclass
class Entry:
    """One pulled page: where it lives locally and the revision it tracks."""

    title: str
    file: str
    revid: int
    host: str
    path: str
    sha256: str = ""  # hash of the pulled wikitext; empty for pre-hash manifests
    pageid: int = 0  # MediaWiki page id; 0 for pre-v2 entries (re-pull to populate)


@dataclass
class Manifest:
    version: int = CURRENT_VERSION
    entries: dict[str, Entry] = field(default_factory=dict)  # keyed by page title

    @classmethod
    def load(cls, root: Path) -> Manifest:
        path = root / MANIFEST_DIR / MANIFEST_FILE
        if not path.exists():
            return cls()
        data = json.loads(path.read_text(encoding="utf-8"))
        # Unknown keys from a newer schema are ignored so an old client fails soft.
        fields = {"title", "file", "revid", "host", "path", "sha256", "pageid"}
        entries = {
            title: Entry(**{k: v for k, v in entry.items() if k in fields})
            for title, entry in data.get("entries", {}).items()
        }
        return cls(version=int(data.get("version", 1)), entries=entries)

    def save(self, root: Path) -> Path:
        """Write the manifest atomically, upgrading it to the current schema."""
        directory = root / MANIFEST_DIR
        directory.mkdir(parents=True, exist_ok=True)
        path = directory / MANIFEST_FILE
        payload = {
            "version": CURRENT_VERSION,
            "entries": {title: asdict(entry) for title, entry in self.entries.items()},
        }
        tmp = path.with_suffix(".json.tmp")
        tmp.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        os.replace(tmp, path)  # atomic on the same filesystem
        self.version = CURRENT_VERSION
        return path
