from collections.abc import Callable
from pathlib import Path

from wikisync.client import FetchedPage
from wikisync.manifest import Entry, Manifest
from wikisync.pull import pull
from wikisync.status import State, status


def _fetch(pages: dict[str, FetchedPage]) -> Callable[[str], FetchedPage]:
    def fetch(title: str) -> FetchedPage:
        return pages[title]

    return fetch


def _pull_one(tmp_path: Path, title: str = "A", text: str = "hello\n") -> None:
    pages = {title: FetchedPage(title=title, text=text, revid=1, exists=True)}
    pull([title], _fetch(pages), root=tmp_path, host="h", path="/w/")


def test_status_unchanged_after_pull(tmp_path: Path):
    _pull_one(tmp_path)
    report = status(tmp_path)
    assert [item.state for item in report.items] == [State.UNCHANGED]
    assert report.modified == []


def test_status_detects_local_modification(tmp_path: Path):
    _pull_one(tmp_path)
    (tmp_path / "A.wiki").write_text("hello world\n", encoding="utf-8")
    report = status(tmp_path)
    assert report.items[0].state is State.MODIFIED
    assert [item.title for item in report.modified] == ["A"]


def test_status_detects_deleted_file(tmp_path: Path):
    _pull_one(tmp_path)
    (tmp_path / "A.wiki").unlink()
    assert status(tmp_path).items[0].state is State.DELETED


def test_status_unknown_when_baseline_hash_missing(tmp_path: Path):
    # Simulate a pre-hash manifest entry (sha256 defaults to "").
    manifest = Manifest()
    manifest.entries["A"] = Entry(title="A", file="A.wiki", revid=1, host="h", path="/w/")
    manifest.save(tmp_path)
    (tmp_path / "A.wiki").write_text("whatever\n", encoding="utf-8")
    assert status(tmp_path).items[0].state is State.UNKNOWN


def test_status_empty_when_nothing_pulled(tmp_path: Path):
    assert status(tmp_path).items == []
