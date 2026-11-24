# Adding a language server

This guide is for someone who wants Coddy to run one more language server, so going to a
definition, hovering and finding uses work in its language. By the end the server starts by itself
the first time a file of its language is opened.

## The shortest way

Make sure the language is in the table, with its `ServerLanguage` (see
[adding a language](1-adding-a-language.md)). Then add the server to `Servers` in
`internal/lsp/servers.go`:

```go
{Name: "zls", Command: "zls", Languages: language.ServerLanguages("Zig"), RootMarkers: []string{"build.zig"},
	Install: "brew install zls"},
```

Open a `.zig` file of a project with a `build.zig`. The servers' panel (the button at the right of the
status bar) shows `zls` starting, then ready.

## How it is run

`internal/servers` runs the table. For a file open, it finds the server whose `Languages` holds the
file's `ServerLanguage`, and its root: the nearest folder above the file with one of `RootMarkers`.
One server runs per root, unless `OnePerWindow` asks for one for the whole window, each project a
workspace folder (as jdtls wants).

The server is told the file open and its edits, and the file left is closed on it. A server nobody
asks for a while rests (it is stopped) and starts again when needed. When its command is not on the
`PATH`, the panel offers `Install`.

## The variations

| Field | Use it when the server… |
|---|---|
| `Arguments`, `Environment` | needs flags or variables to start |
| `DataFlag` | keeps an index in a folder it is given (`-data` for jdtls) |
| `Options`, `Settings` | documents `initializationOptions`, or asks `workspace/configuration` |
| `ReadsAll` | keeps one index per set of projects, so it should read all of them from the start |
| `Ready.Status` | says when it has read the projects; requests wait until then |
| `Memory`, `Index`, `Import` | is a Java virtual machine, keeps indexes, or imports Gradle builds |

Each field has its comment in `servers.go`; jdtls and kotlin-lsp use most of them.

## Checking it

```bash
go test ./internal/lsp ./internal/servers
```

The tests run a fake server (`internal/lsp/fake_test.go`), so they need no real one installed.
