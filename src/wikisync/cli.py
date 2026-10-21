"""Command-line interface for wikisync.

Workflow (git-like, but for wikis):

    wikisync pull <page> [<page> ...]   fetch wiki pages to local wikitext files
    wikisync status                     show local edits against fetched revisions
    wikisync push                       publish staged edits as your wiki account

``pull`` and ``status`` are implemented; ``push`` is a stub for now. The transport
is the MediaWiki API (default https://www.appropedia.org/w/api.php) via mwclient.
Pushing will authenticate as a real user and edit a live public wiki, so it will
be gated behind explicit confirmation when implemented.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from mwclient.errors import MwClientError

from wikisync import __version__
from wikisync.client import fetch_page, open_site
from wikisync.pull import pull
from wikisync.status import status

DEFAULT_HOST = "www.appropedia.org"
DEFAULT_PATH = "/w/"

# Subcommands not yet implemented, mapped to a one-line description for --help.
STUB_COMMANDS: dict[str, str] = {
    "push": "publish local edits as your authenticated wiki account",
}


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="wikisync",
        description="Work with MediaWiki/Appropedia pages locally — pull, edit, push.",
    )
    parser.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    subparsers = parser.add_subparsers(dest="command", metavar="command")

    pull_parser = subparsers.add_parser("pull", help="fetch wiki pages to local wikitext files")
    pull_parser.add_argument("titles", nargs="+", metavar="PAGE", help="page title(s) to fetch")
    pull_parser.add_argument(
        "--host", default=DEFAULT_HOST, help=f"wiki host (default: {DEFAULT_HOST})"
    )
    pull_parser.add_argument(
        "--path", default=DEFAULT_PATH, help=f"API script path (default: {DEFAULT_PATH})"
    )
    pull_parser.add_argument(
        "--dir",
        default=".",
        type=Path,
        help="directory to write files into (default: current directory)",
    )

    status_parser = subparsers.add_parser(
        "status", help="show local edits against the fetched revisions"
    )
    status_parser.add_argument(
        "--dir",
        default=".",
        type=Path,
        help="directory holding the pulled files (default: current directory)",
    )

    for name, help_text in STUB_COMMANDS.items():
        subparsers.add_parser(name, help=help_text)
    return parser


def _run_pull(args: argparse.Namespace) -> int:
    root: Path = args.dir
    root.mkdir(parents=True, exist_ok=True)
    try:
        site = open_site(args.host, path=args.path)
        report = pull(
            args.titles,
            fetch=lambda title: fetch_page(site, title),
            root=root,
            host=args.host,
            path=args.path,
        )
    except (OSError, MwClientError) as exc:
        print(f"wikisync pull: could not reach {args.host}: {exc}", file=sys.stderr)
        return 1

    for title, filename in report.written:
        print(f"pulled {title!r} -> {filename}")
    for title in report.missing:
        print(f"skipped {title!r}: page does not exist", file=sys.stderr)
    return 0 if report.written else 1


def _run_status(args: argparse.Namespace) -> int:
    report = status(args.dir)
    if not report.items:
        print("nothing pulled yet; run 'wikisync pull <page>' first")
        return 0
    for item in report.items:
        print(f"{item.state.value:<16} {item.title} ({item.file})")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.command is None:
        parser.print_help()
        return 0
    if args.command == "pull":
        return _run_pull(args)
    if args.command == "status":
        return _run_status(args)
    print(f"wikisync {args.command}: not yet implemented", file=sys.stderr)
    return 2
