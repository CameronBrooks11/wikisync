"""``wikisync status`` — compare local files against the pulled baseline.

Local only: it reports how the files on disk differ from the revisions recorded
by ``pull`` in the manifest. It does not contact the wiki.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import StrEnum
from pathlib import Path

from wikisync.manifest import Manifest, content_hash


class State(StrEnum):
    UNCHANGED = "unchanged"
    MODIFIED = "modified"
    DELETED = "deleted"
    UNKNOWN = "baseline-unknown"


@dataclass
class StatusItem:
    title: str
    file: str
    state: State


@dataclass
class StatusReport:
    items: list[StatusItem] = field(default_factory=list)

    @property
    def modified(self) -> list[StatusItem]:
        return [item for item in self.items if item.state is State.MODIFIED]


def status(root: Path) -> StatusReport:
    """Report the state of each pulled page's local file against its baseline hash."""
    manifest = Manifest.load(root)
    report = StatusReport()
    for entry in manifest.entries.values():
        file_path = root / entry.file
        if not file_path.exists():
            state = State.DELETED
        elif not entry.sha256:
            state = State.UNKNOWN
        elif content_hash(file_path.read_text(encoding="utf-8")) == entry.sha256:
            state = State.UNCHANGED
        else:
            state = State.MODIFIED
        report.items.append(StatusItem(title=entry.title, file=entry.file, state=state))
    report.items.sort(key=lambda item: item.title)
    return report
