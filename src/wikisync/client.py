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


def open_site(host: str, path: str = "/w/", scheme: str = "https") -> mwclient.Site:
    """Open a MediaWiki site by host (e.g. ``www.appropedia.org``)."""
    return mwclient.Site(host, path=path, scheme=scheme, clients_useragent=USER_AGENT)


def fetch_page(site: mwclient.Site, title: str) -> FetchedPage:
    """Fetch one page's current wikitext. Missing pages come back with ``exists=False``."""
    page = site.pages[title]
    exists = bool(page.exists)
    return FetchedPage(
        title=str(page.name),
        text=page.text() if exists else "",
        revid=int(page.revision or 0),
        exists=exists,
    )
