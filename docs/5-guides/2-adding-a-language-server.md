# Adding a language server

This guide is for someone who wants Coddy to run one more language server, so going to a
definition, hovering and finding uses work in its language. By the end the server starts by itself
the first time a file of its language is opened.

## The shortest way

Make sure the language is in the table, with its `ServerLanguage` (see
[adding a language](1-adding-a-language.md)). Then make a plugin whose program is the server: a
folder in `plugins/` with a `plugin.json`, as `plugins/go/plugin.json` is for gopls:

```json
{
  "name": "zig",
  "version": "0.1.0",
  "description": "Reads Zig with zls.",
  "command": ["zls"],
  "speaks": "lsp",
  "requires": [{ "program": "zls", "install": { "linux": ["brew install zls"], "darwin": ["brew install zls"] } }],
  "contributes": {
    "languageServer": {
      "languages": ["zig"],
      "rootMarkers": ["build.zig"]
    }
  }
}
```

Install it (`plugins/install.sh zig`), start Coddy, and open a `.zig` file of a project with a
`build.zig`. The servers' panel (the button at the right of the status bar) shows `zls`
starting, then ready.

## How it is run

`editor/internal/servers` runs the servers of the plugins on. For a file open, it finds the server whose `Languages` holds the
file's `ServerLanguage`, and its root: the nearest folder above the file with one of `RootMarkers`.
One server runs per root, unless `OnePerWindow` asks for one for the whole window, each project a
workspace folder (as jdtls wants).

The server is told the file open and its edits, and the file left is closed on it. A server nobody
asks for a while rests (it is stopped) and starts again when needed. When its command is not on the
`PATH`, the panel offers `Install`.

## The variations

A server on the Java virtual machine, or one that imports Gradle builds, needs more fields: the
[plugin guide](5-writing-a-plugin.md) lists each, and `plugins/java` and `plugins/kotlin` use most
of them.

## Checking it

```bash
go test ./internal/lsp ./internal/servers
```

The tests run a fake server (`editor/internal/lsp/fake_test.go`), so they need no real one installed.
