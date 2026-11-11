# Contributing

Bug reports, fixes and improvements are welcome.

## Set up

```sh
git clone https://github.com/CameronBrooks11/wikisync.git
cd wikisync
just setup
```

You need Go 1.23+ and [`just`](https://github.com/casey/just). See
[docs/development.md](docs/development.md) for the layout and test layers.

## Make a change

1. Branch from `main`.
2. Make the change, with a test that fails without it.
3. Run `just check && just test`. If you touched the push path and have Docker,
   run `just test-integration` too; CI runs it on every pull request.
4. Open a pull request against `main`.

Never point `just test-live` at a real content page: it edits the page it's
given. Use a page set aside for testing, such as a subpage of your user page.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org): `type(scope):
description`, imperative and lowercase, subject under 72 characters, e.g.
`fix(mw): follow continuation when a page batch is truncated`. Use the body to
say why, and reference issues there.

## Reporting a bug

Open an issue with the command you ran, what you expected, what happened
(including the output), your wikisync version (`wikisync --version`) and the
wiki's host. Leave out credentials.

Security problems go through [SECURITY.md](SECURITY.md), not public issues.

## License

Contributions are licensed under the project's [MIT license](LICENSE).
