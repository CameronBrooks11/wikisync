from pathlib import Path

from wikisync.baseline import baseline_path, read_baseline, write_baseline


def test_baseline_round_trip(tmp_path: Path):
    write_baseline(tmp_path, 42, "hello\n")
    assert read_baseline(tmp_path, 42) == "hello\n"
    assert baseline_path(tmp_path, 42).name == "42.wiki"


def test_read_missing_baseline_returns_none(tmp_path: Path):
    assert read_baseline(tmp_path, 99) is None
