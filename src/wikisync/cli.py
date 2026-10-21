"""Command-line interface for wikisync.

Intended workflow (not yet implemented — this is a scaffold):

    wikisync pull <page|category>   fetch wiki pages to local wikitext files
    wikisync status                 show local edits against the fetched revisions
    wikisync push                   publish staged edits as your wiki account

The transport is the MediaWiki API (e.g. https://www.appropedia.org/w/api.php),
driven via ``mwclient``. Pushing authenticates as a real user and edits a live
public wiki, so it will be gated behind explicit confirmation when implemented.
"""

from __future__ import annotations

import argparse
import sys

from wikisync import __version__

# Planned subcommands, kept here so --help documents the shape before the
# behaviour exists. Each maps to a one-line description.
PLANNED_COMMANDS: dict[str, str] = {
    "pull": "fetch wiki pages to local wikitext files",
    "status": "show local edits against the fetched revisions",
    "push": "publish local edits as your authenticated wiki account",
}


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="wikisync",
        description="Work with MediaWiki/Appropedia pages locally — pull, edit, push.",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"%(prog)s {__version__}",
    )
    subparsers = parser.add_subparsers(dest="command", metavar="command")
    for name, help_text in PLANNED_COMMANDS.items():
        subparsers.add_parser(name, help=help_text)
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.command is None:
        parser.print_help()
        return 0
    print(f"wikisync {args.command}: not yet implemented", file=sys.stderr)
    return 2
