from pathlib import Path

from wikisync.manifest import Entry, Manifest


def test_manifest_round_trip(tmp_path: Path):
    manifest = Manifest()
    entry = Entry(title="Water Pump", file="Water_Pump.wiki", revid=7, host="h", path="/w/")
    manifest.entries["Water Pump"] = entry
    manifest.save(tmp_path)

    loaded = Manifest.load(tmp_path)
    assert loaded.version == 1
    assert loaded.entries["Water Pump"] == entry


def test_manifest_load_missing_returns_empty(tmp_path: Path):
    assert Manifest.load(tmp_path).entries == {}
