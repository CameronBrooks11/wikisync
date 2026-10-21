from collections.abc import Callable
from pathlib import Path

from wikisync.client import FetchedPage
from wikisync.diff import DiffState, diff
from wikisync.manifest import Entry, Manifest
from wikisync.pull import pull


def _fetch(pages: dict[str, FetchedPage]) -> Callable[[str], FetchedPage]:
    def fetch(title: str) -> FetchedPage:
        return pages[title]

    return fetch


def _page(title: str, text: str, pageid: int = 1, revid: int = 1) -> FetchedPage:
    return FetchedPage(title=title, text=text, revid=revid, exists=True, pageid=pageid)


def test_diff_unchanged_after_pull(tmp_path: Path):
    pull(["A"], _fetch({"A": _page("A", "hello\n")}), tmp_path, "h", "/w/")
    (result,) = diff(tmp_path)
    assert result.state is DiffState.UNCHANGED
    assert result.lines == []


def test_diff_shows_unified_diff_for_local_edit(tmp_path: Path):
    pull(["A"], _fetch({"A": _page("A", "line1\nline2\n")}), tmp_path, "h", "/w/")
    (tmp_path / "A.wiki").write_text("line1\nCHANGED\n", encoding="utf-8")

    (result,) = diff(tmp_path)
    assert result.state is DiffState.CHANGED
    body = "".join(result.lines)
    assert "-line2" in body
    assert "+CHANGED" in body


def test_diff_reports_deleted_file(tmp_path: Path):
    pull(["A"], _fetch({"A": _page("A", "x\n")}), tmp_path, "h", "/w/")
    (tmp_path / "A.wiki").unlink()
    (result,) = diff(tmp_path)
    assert result.state is DiffState.DELETED


def test_diff_no_baseline_for_pre_v2_entry(tmp_path: Path):
    manifest = Manifest()
    manifest.entries["A"] = Entry(
        title="A", file="A.wiki", revid=1, host="h", path="/w/", sha256="x", pageid=0
    )
    manifest.save(tmp_path)
    (tmp_path / "A.wiki").write_text("x\n", encoding="utf-8")

    (result,) = diff(tmp_path)
    assert result.state is DiffState.NO_BASELINE


def test_diff_filters_by_title(tmp_path: Path):
    pages = {"A": _page("A", "a\n", pageid=1), "B": _page("B", "b\n", pageid=2)}
    pull(["A", "B"], _fetch(pages), tmp_path, "h", "/w/")
    (tmp_path / "A.wiki").write_text("a2\n", encoding="utf-8")

    results = diff(tmp_path, ["A"])
    assert [d.title for d in results] == ["A"]
