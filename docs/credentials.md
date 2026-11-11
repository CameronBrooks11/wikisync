# Credentials

Only `push --apply` needs credentials. `pull`, `status`, `diff` and the `push`
preview read public pages and never log in.

## Use a bot password, not your login

MediaWiki bot passwords are separate passwords for your account, limited to the
permissions you choose and revocable at any time. Edits made with one are
ordinary edits by your account.

1. Log in to the wiki and open its `Special:BotPasswords` page
   (for example `https://wiki.example.org/wiki/Special:BotPasswords`).
2. Enter a name for the bot password, such as `wikisync`, and create it.
3. Grant **Edit existing pages**. wikisync needs nothing else: it never creates,
   moves or deletes pages.
4. Save. The wiki shows the username to use, `YourName@wikisync`, and a
   password. Copy the password now; it isn't shown again.

## Set the environment variables

wikisync reads the pair for the wiki you're pushing to:

```sh
export WIKISYNC_USERNAME_EXAMPLE='YourName@wikisync'
export WIKISYNC_PASSWORD_EXAMPLE='…'
```

The suffix comes from the host: the label before the top-level domain,
uppercased, with anything other than letters and digits turned into `_`.

| Wiki host | Variables |
|---|---|
| `wiki.example.org` | `WIKISYNC_USERNAME_EXAMPLE`, `WIKISYNC_PASSWORD_EXAMPLE` |
| `en.wikipedia.org` | `WIKISYNC_USERNAME_WIKIPEDIA`, `WIKISYNC_PASSWORD_WIKIPEDIA` |
| `wiki.my-lab.net` | `WIKISYNC_USERNAME_MY_LAB`, `WIKISYNC_PASSWORD_MY_LAB` |

If that pair isn't set, wikisync falls back to `WIKISYNC_USERNAME` and
`WIKISYNC_PASSWORD`. Both halves of a pair must come from the same source; a
username from one and a password from the other is never used. Use the generic
names for a bare IP address, or for hosts under a two-part suffix such as
`.co.uk`, where the label rule picks `CO`.

`push --apply` prints which variable it logged in from, never the password.

## Keep the password out of files

Set the variables in your shell or load them from a password or secrets
manager for the one command. Don't put them in a file inside the working tree:
it may end up in version control or a backup. If a bot password leaks, revoke
it on `Special:BotPasswords`; your account password is unaffected.
