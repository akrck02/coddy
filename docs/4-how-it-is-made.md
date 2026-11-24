# How it is made

This page is for someone who wants to understand how Coddy works inside before changing it. By
the end you will know how its packages lean on each other, how a frame is drawn, which work runs
off the frame, and what is kept between frames and why.

## 🧱 The packages, in layers

Think of the code as a building: each floor rests on the ones below and never on those above.
A package only imports packages of its own floor or lower, so you can read the code from the
bottom up and never meet a name you have not seen yet.

```
cmd/code-review                                        the program: flags, preferences, wiring
files · review · uses                                  the modes
window                                                 the shell: layout, focus, keys, overlays
navigation · servers · session                         what the window does besides drawing
mode                                                   what a mode is, and what it may do
viewer · sidebar · finder · explorer · settings        the panes
theme · pane                                           what every pane shares
workspace · java · lsp · gradle · builds · text · highlight · markdown     the domain
git · language · work · english                        the base
```

All of them live in `internal/`, but for the program. The [contributing guide](8-contributing.md)
says in one line what each one holds; `cmd/code-review/rules_test.go` checks the floors that
matter most: no pane, and none of `navigation`, `servers`, `session` or `work`, imports `window`;
the window imports no mode; the domain imports nothing of the interface; the language table
imports nothing at all.

Why so many floors? Because each one can be tested alone. `session` reads projects without a
window. `servers` runs language servers for a fake host. A pane draws for a test window that
answers a handful of questions.

## 🖼️ How a frame is built

Go UI is *immediate mode*: it asks the program to describe the whole window again at every
frame, as a sheet of paper redrawn from scratch rather than a board whose pieces are moved. The
program's `Window.Build` (`internal/window/build.go`) is that description. It runs in this order:

1. **Take what came back.** `work.Take` applies the results of the background jobs that finished
   since the last frame (a repository read, a server's answer).
2. **Handle the keys.** Commands run, the open file takes what was typed, the folds follow the
   caret.
3. **Tell the servers.** `servers.Serve` gets the file open and tells its server only if the text
   changed since it was told; `navigation.Uses.Advance` moves a search of uses a step on.
4. **Describe the panes.** The mode rail, the sidebar, the title bar and the viewer (or the
   explorer, or a mode's own page), then the status bar.
5. **Draw the overlays.** The dialogs over the window are one list, `overlaysOf`
   (`internal/window/overlays.go`): the finder, the base dialog, the servers' panel, the picker,
   the questions. Each says whether it is open and how it is drawn. `ModalOpen` walks the same
   list, so a new dialog is one line, and the panes stop taking keys while it is up.
6. **Notes and toasts** go last, on top of everything.

The sidebar, the code and the finder's results are not boxes of Go UI but lists the program
paints itself (`Context.Paint`), drawing only the rows in sight. That is how a long file scrolls as
fast as a short one.

## 🧵 The threads

Only one goroutine touches the interface: the one that builds frames. Anything slow (reading a
repository, asking a server, searching) runs elsewhere, through a `work.Queue`
(`internal/work/work.go`):

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

## 🗄️ What is kept between frames, and why

A frame that does nothing new should cost almost nothing. The rule is: **work that only changes
with data is done when the data changes, not each time it is drawn.**
`internal/window/idle_test.go` holds the line: an idle frame may make at most 40 allocations in
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
| The documents read | `session` | the file's time or size on disk changed, checked when it is opened |
| What each server was told | `servers` | the open file's text changed |

Two of these deserve a word more.

**The documents** are kept up to about 64 MB (`documentCacheBytes` in
`internal/session/cache.go`), and the least recently opened go first. Their size is an estimate
from their text and lines, not a measure. Each remembers its file's time and size, so a file
changed by another program is read again when it is opened, never in a frame.

**A language server** is told the file open and nothing else. When another file is opened, the
old one is closed on the server (`didClose`), so a server never holds every file a person
browsed.

## 📦 Where the libraries come from

The window is drawn by [Go UI](https://github.com/akrck02/go-ui) on
[Go Renderer](https://github.com/akrck02/go-renderer), and the code is coloured by
[Chroma](https://github.com/alecthomas/chroma). Bubble's `goui` module points at its neighbours
through local paths, so `go.mod` replaces Bubble's and Go UI's `v0.0.0` with real versions.
