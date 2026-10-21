import json
from pathlib import Path

from wikisync.manifest import CURRENT_VERSION, MANIFEST_DIR, MANIFEST_FILE, Entry, Manifest


def test_manifest_round_trip(tmp_path: Path):
    manifest = Manifest()
    entry = Entry(title="Water Pump", file="Water_Pump.wiki", revid=7, host="h", path="/w/")
    manifest.entries["Water Pump"] = entry
    manifest.save(tmp_path)

    loaded = Manifest.load(tmp_path)
    assert loaded.version == CURRENT_VERSION
    assert loaded.entries["Water Pump"] == entry


def test_manifest_load_missing_returns_empty(tmp_path: Path):
    assert Manifest.load(tmp_path).entries == {}


def test_manifest_loads_v1_without_pageid(tmp_path: Path):
    # A v1 manifest predates the pageid field; it must still load, defaulting to 0.
    directory = tmp_path / MANIFEST_DIR
    directory.mkdir()
    (directory / MANIFEST_FILE).write_text(
        json.dumps(
            {
                "version": 1,
                "entries": {
                    "A": {
                        "title": "A",
                        "file": "A.wiki",
                        "revid": 3,
                        "host": "h",
                        "path": "/w/",
                        "sha256": "abc",
                    }
                },
            }
        ),
        encoding="utf-8",
    )

    loaded = Manifest.load(tmp_path)
    assert loaded.version == 1
    assert loaded.entries["A"].pageid == 0
    assert loaded.entries["A"].sha256 == "abc"
