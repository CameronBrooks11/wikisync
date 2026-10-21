import pytest

from wikisync import __version__
from wikisync.cli import STUB_COMMANDS, build_parser, main


def test_version_is_set():
    assert __version__ == "0.1.0"


def test_no_command_prints_help_and_succeeds(capsys):
    assert main([]) == 0
    assert "wikisync" in capsys.readouterr().out


def test_version_flag_exits_zero(capsys):
    # argparse's `version` action exits via SystemExit(0).
    with pytest.raises(SystemExit) as exc:
        main(["--version"])
    assert exc.value.code == 0
    assert __version__ in capsys.readouterr().out


@pytest.mark.parametrize("command", sorted(STUB_COMMANDS))
def test_stub_command_reports_not_yet_implemented(command, capsys):
    assert main([command]) == 2
    assert "not yet implemented" in capsys.readouterr().err


def test_pull_parser_requires_a_title():
    with pytest.raises(SystemExit):
        build_parser().parse_args(["pull"])  # nargs="+" → at least one title required


def test_pull_parser_accepts_titles_and_options():
    args = build_parser().parse_args(["pull", "A", "B", "--host", "example.org"])
    assert args.command == "pull"
    assert args.titles == ["A", "B"]
    assert args.host == "example.org"


def test_status_parser_accepts_dir():
    args = build_parser().parse_args(["status", "--dir", "/tmp/x"])
    assert args.command == "status"


def test_status_on_empty_dir_succeeds(tmp_path, capsys):
    assert main(["status", "--dir", str(tmp_path)]) == 0
    assert "nothing pulled yet" in capsys.readouterr().out
