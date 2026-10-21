"""Thin wrapper over mwclient for reading MediaWiki pages.

Writing (push) will live here too once implemented; for now this is read-only.
"""

from __future__ import annotations

from dataclasses import dataclass

import mwclient

from wikisync import __version__

# Honest, app-identifying User-Agent, as MediaWiki etiquette expects. The repo
# is public, so the URL carries no private information.
USER_AGENT = f"wikisync/{__version__} (+https://github.com/CameronBrooks11/wikisync)"


@dataclass(frozen=True)
class FetchedPage:
    """A page's current wikitext and the revision it came from."""

    title: str
    text: str
    revid: int
    exists: bool
    pageid: int = 0


def open_site(host: str, path: str = "/w/", scheme: str = "https") -> mwclient.Site:
    """Open a MediaWiki site by host (e.g. ``www.appropedia.org``)."""
    return mwclient.Site(host, path=path, scheme=scheme, clients_useragent=USER_AGENT)


def fetch_page(site: mwclient.Site, title: str) -> FetchedPage:
    """Fetch one page's current wikitext and the revision it came from.

    Content and revision id are read from the *same* revisions response, so the
    recorded baseline revid always matches the saved text. (Reading them from
    separate requests can skew if the page is edited in between.) Missing pages
    come back with ``exists=False``.
    """
    resp = site.api(
        "query",
        prop="revisions",
        titles=title,
        rvprop="ids|content",
        rvslots="main",
        rvlimit=1,
        formatversion=2,
    )
    page = resp["query"]["pages"][0]
    if page.get("missing", False):
        return FetchedPage(title=page.get("title", title), text="", revid=0, exists=False)
    revision = page["revisions"][0]
    return FetchedPage(
        title=page["title"],
        text=revision["slots"]["main"]["content"],
        revid=int(revision["revid"]),
        exists=True,
        pageid=int(page["pageid"]),
    )
