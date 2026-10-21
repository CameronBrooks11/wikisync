from collections.abc import Callable
from pathlib import Path

from wikisync.client import FetchedPage
from wikisync.manifest import Entry, Manifest
from wikisync.pull import pull, title_to_filename


def _fake_fetch(pages: dict[str, FetchedPage]) -> Callable[[str], FetchedPage]:
    def fetch(title: str) -> FetchedPage:
        return pages[title]

    return fetch


def _page(title: str, text: str, revid: int = 1) -> FetchedPage:
    return FetchedPage(title=title, text=text, revid=revid, exists=True)


def test_title_to_filename_is_reversible_and_collision_safe():
    assert title_to_filename("Open Source Water Pump") == "Open_Source_Water_Pump.wiki"
    # Distinct titles that used to collapse to the same name now differ.
    assert title_to_filename("Foo/Bar") == "Foo%2FBar.wiki"
    assert title_to_filename("Foo-Bar") == "Foo-Bar.wiki"
    assert title_to_filename("Foo/Bar") != title_to_filename("Foo-Bar")
    # Space and underscore are the same MediaWiki title, so they share a name.
    assert title_to_filename("A B") == title_to_filename("A_B") == "A_B.wiki"


def test_pull_writes_files_and_manifest(tmp_path: Path):
    report = pull(
        ["Water Pump"],
        _fake_fetch({"Water Pump": _page("Water Pump", "== Build ==\n", revid=42)}),
        root=tmp_path,
        host="www.appropedia.org",
        path="/w/",
    )

    assert report.written == [("Water Pump", "Water_Pump.wiki")]
    assert (tmp_path / "Water_Pump.wiki").read_text(encoding="utf-8") == "== Build ==\n"

    entry = Manifest.load(tmp_path).entries["Water Pump"]
    assert entry.revid == 42
    assert entry.sha256  # baseline hash recorded


def test_pull_records_missing_and_partial(tmp_path: Path):
    pages = {
        "Real": _page("Real", "x\n"),
        "Ghost": FetchedPage(title="Ghost", text="", revid=0, exists=False),
    }
    report = pull(["Real", "Ghost"], _fake_fetch(pages), root=tmp_path, host="h", path="/w/")
    assert [t for t, _ in report.written] == ["Real"]
    assert report.missing == ["Ghost"]
    assert not (tmp_path / "Ghost.wiki").exists()


def test_pull_refuses_to_overwrite_dirty_file(tmp_path: Path):
    pull(["A"], _fake_fetch({"A": _page("A", "original\n")}), root=tmp_path, host="h", path="/w/")
    (tmp_path / "A.wiki").write_text("my local edit\n", encoding="utf-8")

    report = pull(["A"], _fake_fetch({"A": _page("A", "newer remote\n")}), tmp_path, "h", "/w/")

    assert report.skipped_dirty == ["A"]
    assert report.written == []
    # The local edit is preserved, not clobbered.
    assert (tmp_path / "A.wiki").read_text(encoding="utf-8") == "my local edit\n"


def test_pull_force_overwrites_dirty_file(tmp_path: Path):
    pull(["A"], _fake_fetch({"A": _page("A", "original\n")}), root=tmp_path, host="h", path="/w/")
    (tmp_path / "A.wiki").write_text("my local edit\n", encoding="utf-8")

    report = pull(
        ["A"], _fake_fetch({"A": _page("A", "newer remote\n")}), tmp_path, "h", "/w/", force=True
    )

    assert report.written == [("A", "A.wiki")]
    assert (tmp_path / "A.wiki").read_text(encoding="utf-8") == "newer remote\n"


def test_pull_refreshes_clean_file(tmp_path: Path):
    pull(
        ["A"], _fake_fetch({"A": _page("A", "v1\n", revid=1)}), root=tmp_path, host="h", path="/w/"
    )
    # File untouched since pull → re-pull updates it to the newer remote content.
    report = pull(["A"], _fake_fetch({"A": _page("A", "v2\n", revid=2)}), tmp_path, "h", "/w/")

    assert report.written == [("A", "A.wiki")]
    assert (tmp_path / "A.wiki").read_text(encoding="utf-8") == "v2\n"
    assert Manifest.load(tmp_path).entries["A"].revid == 2


def test_pull_detects_filename_collision(tmp_path: Path):
    # A manifest where some other title already owns the filename "Foo.wiki".
    manifest = Manifest()
    manifest.entries["Other"] = Entry(
        title="Other", file="Foo.wiki", revid=1, host="h", path="/w/", sha256="deadbeef"
    )
    manifest.save(tmp_path)

    report = pull(["Foo"], _fake_fetch({"Foo": _page("Foo", "x\n")}), tmp_path, "h", "/w/")

    assert report.collisions == [("Foo", "Foo.wiki", "Other")]
    assert report.written == []
    assert not (tmp_path / "Foo.wiki").exists()
