from typing import Any

from wikisync.client import fetch_page


class FakeSite:
    """Minimal stand-in exposing the one method fetch_page uses."""

    def __init__(self, response: dict[str, Any]):
        self._response = response
        self.calls: list[dict[str, Any]] = []

    def api(self, action: str, **params: Any) -> dict[str, Any]:
        self.calls.append({"action": action, **params})
        return self._response


def test_fetch_page_reads_text_and_revid_from_the_same_revision():
    # One revisions response carrying both the id and the content.
    response = {
        "query": {
            "pages": [
                {
                    "pageid": 5,
                    "title": "Water Pump",
                    "revisions": [{"revid": 99, "slots": {"main": {"content": "hello\n"}}}],
                }
            ]
        }
    }
    site: Any = FakeSite(response)
    page = fetch_page(site, "Water Pump")

    assert page.exists
    assert page.title == "Water Pump"
    assert page.revid == 99
    assert page.pageid == 5
    assert page.text == "hello\n"
    # Content and id came from a single query asking for both.
    assert site.calls[0]["prop"] == "revisions"
    assert "ids" in site.calls[0]["rvprop"] and "content" in site.calls[0]["rvprop"]


def test_fetch_page_missing():
    response = {"query": {"pages": [{"title": "Ghost", "missing": True}]}}
    site: Any = FakeSite(response)
    page = fetch_page(site, "Ghost")

    assert not page.exists
    assert page.revid == 0
    assert page.pageid == 0
    assert page.text == ""
