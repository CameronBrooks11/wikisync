from collections.abc import Callable
from pathlib import Path

from wikisync.client import FetchedPage
from wikisync.manifest import Manifest
from wikisync.pull import pull, title_to_filename


def test_title_to_filename_sanitizes():
    assert title_to_filename("Open Source Water Pump") == "Open_Source_Water_Pump.wiki"
    assert title_to_filename("A/B:C?") == "A-B-C-.wiki"


def _fake_fetch(pages: dict[str, FetchedPage]) -> Callable[[str], FetchedPage]:
    def fetch(title: str) -> FetchedPage:
        return pages[title]

    return fetch


def test_pull_writes_files_and_manifest(tmp_path: Path):
    pages = {
        "Water Pump": FetchedPage(title="Water Pump", text="== Build ==\n", revid=42, exists=True),
    }
    report = pull(
        ["Water Pump"],
        _fake_fetch(pages),
        root=tmp_path,
        host="www.appropedia.org",
        path="/w/",
    )

    assert report.written == [("Water Pump", "Water_Pump.wiki")]
    assert report.missing == []
    assert (tmp_path / "Water_Pump.wiki").read_text(encoding="utf-8") == "== Build ==\n"

    entry = Manifest.load(tmp_path).entries["Water Pump"]
    assert entry.revid == 42
    assert entry.file == "Water_Pump.wiki"
    assert entry.host == "www.appropedia.org"


def test_pull_records_missing_pages(tmp_path: Path):
    pages = {"Ghost": FetchedPage(title="Ghost", text="", revid=0, exists=False)}
    report = pull(["Ghost"], _fake_fetch(pages), root=tmp_path, host="h", path="/w/")

    assert report.missing == ["Ghost"]
    assert report.written == []
    assert not (tmp_path / "Ghost.wiki").exists()
