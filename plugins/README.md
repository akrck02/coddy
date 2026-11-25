# Coddy's plugins

Each folder is one plugin: its `plugin.json`, and the program it runs when that is not already on
the PATH. They are a Go module of their own, so Coddy itself never depends on them; they speak to
Coddy only through its sdk, `github.com/akrck02/coddy/sdk`.

The **official** plugins are Coddy's own capabilities, kept out of its core. Without them Coddy
runs no language server:

- `go` reads Go with gopls.
- `java` reads Java with jdtls.
- `kotlin` reads Kotlin with kotlin-lsp.
- `azure-devops` lists the active pull requests of an Azure DevOps repository and their comments,
  through its REST API. Its token is a setting, kept in the system's keyring (or else
  `AZURE_DEVOPS_EXT_PAT`); another setting points it to Azure DevOps Server on your own machine.

The language plugins' program is a language server written for any editor (`"speaks": "lsp"`),
so each is only a `plugin.json`. Git is official too, but compiled into Coddy: Settings › Plugins lists it as built
in.

The logos of `go`, `kotlin`, `python` and `github` (their `icon.svg`), and git's, compiled into
Coddy, come from [Simple Icons](https://simpleicons.org) (CC0), each in its own colour; GitHub's
is grey, to read on light and dark themes. Those of `java` and `azure-devops` come from
[Devicon](https://devicon.dev) (MIT); Azure DevOps' gradient is flattened to one blue. The logos
remain the trademarks of their owners, used here only to name what each plugin reads.

The **examples** show what a plugin can add:

- `github` lists a GitHub repository's pull requests and their comments, by running `gh`. Its
  one setting chooses which: every open one, yours, or those asking for your review.
- `python` reads Python with Pyright, as the official language plugins do.

To install them, run `plugins/install.sh` (every plugin) or `plugins/install.sh go github` (only
those). It builds each plugin's program and copies it, with its `plugin.json` and its pictures, into
Coddy's plugins folder: `~/.config/coddy/plugins/` on Linux, `~/Library/Application Support/coddy/plugins/` on a
Mac.

For now Coddy's own packages use changes of go-renderer and go-ui not released yet, so the script
builds with the go-renderer and go-ui found beside Coddy's folder when they are there.

A plugin installed is on; Settings › Plugins turns it off. How to write one is in
[docs/5-guides/5-writing-a-plugin.md](../docs/5-guides/5-writing-a-plugin.md).
