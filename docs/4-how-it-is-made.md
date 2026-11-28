# How it is made

This page is for someone who wants to understand how Coddy works inside before changing it. By
the end you will know how its packages lean on each other, how a frame is drawn, which work runs
off the frame, and what is kept between frames and why.

## 🧱 The packages, in layers

Think of the code as a building: each floor rests on the ones below and never on those above.
A package only imports packages of its own floor or lower, so you can read the code from the
bottom up and never meet a name you have not seen yet.

```
cmd/coddy                                               the program: flags, preferences, wiring
ui/mode/files · review · uses · pluginmode              the modes
ui/window                                               the shell: layout, focus, keys, overlays
ui/navigation                                           following names, across the panes
ui/mode                                                 what a mode is, and what it may do
ui/viewer · sidebar · finder · settings                 the panes
ui/shortcut · controls · pane                           what every pane shares
ui/theme                                                the colours and the icons
────────────────────────────────────────────────────────the interface above, the domain below
servers · workspace · plugins                           what the window runs and keeps open
codebase · java · lsp · gradle · builds · text          the domain
format/highlight · format/markdown                      formatting text
git · language · format · execution · plugins/keyring   the base
```

All of them live in `editor/internal/`, but for the program in `editor/cmd/coddy`. The
repository holds two more Go modules beside the editor. `sdk/` is what a plugin is written with:
the `sdk` package is the protocol plugins speak, and `sdk/jsonrpc` the connection both sides of it
read and write through. Coddy talks to language servers and to plugins with the same
`jsonrpc.Conn`, and a plugin answers with it too, so one connection is written and tested once.
The official plugins are the third module, `plugins/`, built on the sdk alone. The editor and the
plugins both depend on the sdk and never on each other, and Go keeps them from reaching into the
editor's `editor/internal/`. The [contributing guide](8-contributing.md)
says in one line what each one holds; `editor/cmd/coddy/rules_test.go` checks the floors that
matter most: no pane, and nothing outside `editor/internal/ui/` imports anything inside it, and no pane imports `window`;
the window imports no mode; the domain imports nothing of the interface; the language table
imports nothing at all.

Why so many floors? Because each one can be tested alone. `workspace` reads projects without a
window. `servers` runs language servers for a fake host. A pane draws for a test window that
answers a handful of questions.

## 🖼️ How a frame is built

Go UI is *immediate mode*: it asks the program to describe the whole window again at every
frame, as a sheet of paper redrawn from scratch rather than a board whose pieces are moved. The
program's `Window.Build` (`editor/internal/ui/window/build.go`) is that description. It runs in this order:

1. **Take what came back.** `work.Take` applies the results of the background jobs that finished
   since the last frame (a repository read, a server's answer).
2. **Handle the keys.** Commands run, the open file takes what was typed, the folds follow the
   caret.
3. **Tell the servers.** `servers.Serve` gets the file open and tells its server only if the text
   changed since it was told; `navigation.Uses.Advance` moves a search of uses a step on.
4. **Describe the panes.** The mode rail, the sidebar, the title bar and the viewer (or the
   hint of a folder chosen in the sidebar, or a mode's own page), then the status bar.
5. **Draw the overlays.** The dialogs over the window are one list, `overlaysOf`
   (`editor/internal/ui/window/overlays.go`): the finder, the base dialog, the servers' panel,
   the questions. Each says whether it is open and how it is drawn. `ModalOpen` walks the same
   list, so a new dialog is one line, and the panes stop taking keys while it is up.
6. **Notes and toasts** go last, on top of everything.

The sidebar, the code and the finder's results are not boxes of Go UI but lists the program
paints itself (`Context.Paint`), drawing only the rows in sight. That is how a long file scrolls as
fast as a short one.

## 🧵 The threads

Only one goroutine touches the interface: the one that builds frames. Anything slow (reading a
repository, asking a server, searching) runs elsewhere, through a `work.Queue`
(`editor/internal/execution/work/work.go`):

- `Run(key, job)` starts the job on its own goroutine. The job returns a small function, and that
  function runs on the interface's goroutine at the next `Take`. So a job never changes the
  window's state while a frame reads it.
- A key names a job. A newer job with the same key cancels the older one through its
  `context.Context`, and the older result is thrown away. Asking for a definition twice in a row
  runs under the key `"definition"`, so only the second answer is ever shown.
- `Read` lets at most six repositories be read at once, so a big workspace on a slow disk does
  not start dozens of git processes together.
- When a result comes back the queue calls `wake`, which asks the window for a frame. The window
  draws only when something happens, so without this the result would wait for the next key.

In tests and captures the queue is *synchronous*: every job runs at once, and the one frame shows
its result.

## 🌿 Git, and other version controls

Everything Coddy knows of a development comes from its version control, read off the frame. Git
is built in (`editor/internal/git`, which runs the `git` command). `codebase.VersionControl` is the
little contract a project can also be read through, six questions (the branch, the default base, the files, the changes, a file at a
revision, a file's diff). `codebase.Git` answers them with git; a plugin can answer them for
another version control, over the sdk's `versionControl/…` messages. `workspace` decides which
one reads each folder opened: git where there is a `.git`, else the plugin whose marker is there.
A folder none of them reads is a plain folder, read by `codebase.FolderControl`: every file on
disk, no change. When such a folder holds repositories and loose files too, it is a project of its
own that leaves the repositories' folders out (`LeftOut`); each repository keeps the folder's path
in `Project.Root`, and the sidebar draws the repository's rows where its folder would be. A list of
changes only has no use for places, so there the repositories are listed flat, as always.
Git is still listed as a plugin, a built-in one (`plugins.BuiltIn`), so a person can turn it off
like any other; turned off, `workspace` reads no `.git`. It stays compiled in: no program between,
the same speed.

Why so few questions? A contract asks only what every version control can give. Git gives more
(the lines each file adds and removes, a content id that marks a reviewed file, the commits
ahead), and `LoadProject` still asks git for those directly; a project read by a plugin goes
without them for now.

## 🗄️ What is kept between frames, and why

A frame that does nothing new should cost almost nothing. The rule is: **work that only changes
with data is done when the data changes, not each time it is drawn.**
`editor/internal/ui/window/idle_test.go` holds the line: an idle frame may make at most 40 allocations in
the files mode and 60 in the review mode, whatever the size of the file and the workspace. Most of
those are Go UI's own and the test window's screen reader, not Coddy's.

What is kept, and what makes it stale:

| What | Where | Made again when |
|---|---|---|
| Labels, counts, the branch, line numbers | `pane.Words` | the value they are made from changes |
| Rows, their decorations and badges, the header | `sidebar.Rebuild` | the projects, the filter or the mode change (`Rebuilds` counts it) |
| The trees per filter typed | `sidebar` | kept for the last 4 filters, so Backspace is instant |
| The command table | `window.startModes` | never: it is made once, when the window starts |
| Markdown laid out | `viewer` | the document or its edit version changes |
| The documents read | `workspace` | the file's time or size on disk changed, checked when it is opened |
| What each server was told | `servers` | the open file's text changed |
| The servers' views (status bar, panel) | `servers.Fleet.Views` | a server says something new (`lsp.Client.Version`), one starts, stops or rests, the projects change; each second while one is at work |

Two of these deserve a word more.

**The documents** are kept up to a budget a person chooses in *Settings › Memory* (a slider from 32
to 512 MB in steps of 32; `DefaultDocumentMegabytes`, 64, until then, in `editor/internal/workspace/cache.go`), and the
least recently opened go first, at once when the budget shrinks. A file of code takes about 100
to 350 KB once read: its two versions, its diff and its colours weigh far more than its text. Their size is an estimate
from their text and lines, not a measure. Each remembers its file's time and size, so a file
changed by another program is read again when it is opened, never in a frame.

**The language servers** come from the plugins on whose program speaks lsp: `lsp.Server` says
how to run one, and the plugin's `plugin.json` fills it in (`plugins.Manifest.Server`). A plugin
is always a program Coddy talks to; a language server is simply one whose protocol Coddy knows
already, so its plugin needs to say only how to run it. Coddy keeps no table of its own; Go, Java
and Kotlin are its official plugins, in `plugins/`, read at start.

**A language server** is told the file open and nothing else. When another file is opened, the
old one is closed on the server (`didClose`), so a server never holds every file a person
browsed.

## 📦 Where the libraries come from

The window is drawn by [Go UI](https://github.com/akrck02/go-ui) on
[Go Renderer](https://github.com/akrck02/go-renderer), and the code is coloured by
[Chroma](https://github.com/alecthomas/chroma). Bubble's `goui` module points at its neighbours
through local paths, so `go.mod` replaces Bubble's and Go UI's `v0.0.0` with real versions.
