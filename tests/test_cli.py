import pytest

from wikisync import __version__
from wikisync.cli import PLANNED_COMMANDS, build_parser, main


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


@pytest.mark.parametrize("command", sorted(PLANNED_COMMANDS))
def test_planned_command_reports_not_yet_implemented(command, capsys):
    assert main([command]) == 2
    assert "not yet implemented" in capsys.readouterr().err


@pytest.mark.parametrize("command", sorted(PLANNED_COMMANDS))
def test_parser_accepts_each_planned_command(command):
    args = build_parser().parse_args([command])
    assert args.command == command


def test_parser_rejects_unknown_command():
    with pytest.raises(SystemExit):
        build_parser().parse_args(["definitely-not-a-command"])
