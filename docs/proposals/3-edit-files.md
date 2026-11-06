# 3. Edit files, with an optional read-only mode

> Status: done, R7 in part (see Result) · Author's approval: proposal ☑ plan ☑

## Why

Code Review was built to only read, and that rule shaped it: `internal/git` refuses any command
that writes, and nothing else touches a repository. The author has changed the rule (6 November
2026): people should be able to fix what they find while they read, and to use language servers
(proposal 4), which need a file that can change. "Read-only" stays, but as a mode of the interface
that someone chooses, not as what the program is.

Two things in today's reader stand in the way. The file view has no caret: it scrolls and marks a
line, nothing more. And most shortcuts are single letters (`n`, `d`, `v`, `j`…), which, once the
file takes the keyboard, have to type those letters instead.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | A file opened whole can be edited: a caret, a selection, typing, deleting, cut, copy, paste, undo and redo, and saving. | Tests on the text buffer (T1, T2) and in the window, typing into a file and saving it (T5). |
| R2 | Saving writes only the file open, all at once, and never over a change made on disk since it was read. | `TestSavingWritesTheWholeFileAtOnce`, `TestSavingRefusesAFileChangedOnDisk`. |
| R3 | A read-only mode stops typing, pasting and saving, and says it is on. | `TestReadOnlyModeTypesNothing`, `TestReadOnlyModeSavesNothing`. |
| R4 | Nothing else writes inside a repository: one function saves a file, and only it. | `TestOnlyTheSaveWritesInARepository`, which looks for file writes outside it. |
| R5 | A plain letter typed in a file being edited types; no shortcut takes it. Every command still has a key that works while editing. | `TestNoPlainKeyCommandWhileEditing`, and the help lists both keys. |
| R6 | What was saved shows as a change: the diff, the tree and the counts read it again. | `TestASavedFileShowsAsChanged`. |
| R7 | A file with changes not saved is not lost: closing it or the window asks first. | `TestClosingAnEditedFileAsks`. |
| R8 | Git still only reads: no commit, no push, no checkout. | `TestOnlyReadOnlyCommandsRun`, unchanged. |

## Out of scope

- Committing, pushing or any other git command that writes. If it is ever wanted, its own
  proposal designs it.
- Editing in the diff view. The diff stays for reading; `d` goes to the whole file to edit it.
- Language servers: proposal 4, which builds on this one.
- Several files open at once (tabs), split views, find and replace, multiple carets.
- Creating, renaming or deleting files.

## Design

### A text buffer of its own

go-ui's text editor is the field's, unexported, and keeps the whole text as one run of characters
with a snapshot of it for every undo. That suits a field, not a file of thousands of lines. A new
package, `internal/text`, holds a file being edited:

- **Lines of characters**, so an edit touches one line, and positions as (line, column) counted in
  characters.
- **A caret and an anchor**: the selection runs between them, as in go-ui.
- **Undo as edits, not snapshots**: each edit keeps what it replaced and where. Typing a word or
  deleting one is undone in one go, as go-ui does.
- **Movement**: by character, word, line, page, to the start and end of a line and of the file.

`text` imports nothing of the interface, like `workspace`, and a rules test checks it.

### Where editing happens

The viewer's whole-file view gains the caret. The diff view stays read-only. When the viewer has
the keyboard, a click places the caret; Shift and a click or drag selects.

The colours of the code come from Chroma over the whole file. After an edit, the lines are coloured
again in the background a moment after typing stops, through `work.Run`, as a search is; until
then the line edited keeps its old colours.

### Saving

`workspace.Save(project, path, text, read)` is the one function that writes in a repository:

- It writes to a temporary file beside the original and renames it over it, so a file is never left
  half written. It keeps the file's permissions.
- It first compares the file on disk with what was read (`read` carries its size, time and a hash).
  If someone changed it since, it refuses with `workspace.changed-on-disk`, and the window asks:
  keep mine, take theirs, or cancel.
- After saving, the project's changes are read again, so the tree, the counts and the diff show the
  edit (R6).

`TestOnlyTheSaveWritesInARepository` reads every Go file and fails on a file write (`os.WriteFile`,
`os.Create`, `os.OpenFile` with writing, `os.Rename`, `os.Remove`) outside `workspace/save.go` and
the review mode's marks, which are kept in the reader's own folder, never in a repository.

### Keys while editing

Commands today have three places: anywhere, the sidebar, the viewer. A fourth, **editing**, is the
viewer with a file being edited. There:

- A plain letter, a digit, a symbol, Space, Enter, Backspace, Tab and the arrows edit the text.
- Commands work only through keys with ⌘ (or Ctrl), F-keys or Esc. Every command that today has only
  a plain letter gains one, for example ⌘N / ⌘M for the next and previous changed file, ⌘D for the
  diff, ⌘⇧V to mark viewed. The help shows both.
- **Esc** stops editing and gives the keyboard to the sidebar, where the letters work as today.

`TestNoPlainKeyCommandWhileEditing` goes through every command and fails if one would take a plain
key while editing.

| Edit | Keys |
|---|---|
| Save | ⌘S |
| Undo, redo | ⌘Z, ⌘⇧Z |
| Cut, copy, paste | ⌘X, ⌘C, ⌘V |
| Select all | ⌘A |
| Word left, right | ⌥←, ⌥→ |
| Start, end of the line | ⌘←, ⌘→ or Home, End |
| Start, end of the file | ⌘↑, ⌘↓ |
| Stop editing | Esc |

### The read-only mode

A switch in the title bar, with a lock icon, and the key ⌘⇧L. While it is on, the file has no caret
for typing (it scrolls and marks lines, as today), and typing, pasting and saving do nothing. The
status bar says "read-only". The choice is remembered, like "only the projects with changes", and
`-read-only` starts with it on.

A file with changes not saved shows a dot by its name in the header and in the tree. Closing it,
opening another or closing the window asks to save, discard or cancel (R7).

### Errors

New codes in `text/errors.go` and `workspace/errors.go`, each with its test: `workspace.changed-on-disk`,
`workspace.not-saved`, `text.position-out-of-range`.

### Documentation

`docs/8-contributing.md` and the README change "the one rule": the reader writes a file only when
someone saves it, through `workspace.Save`, and git still only reads. The help gains the keys.

### Decided with the author

1. **The mode on start** is editing; read-only is remembered once chosen.
2. **The second key of each command** is as above, with ⌘⇧ where ⌘ alone belongs to macOS (⌘H
   hides the window, so "only the projects with changes" is ⌘⇧H).

## Tasks

- [x] T1 — `internal/text`: lines, caret, selection, movement · serves R1 · test: positions and movement, words, an empty file, a file without a final newline
- [x] T2 — `internal/text`: edits and undo · serves R1 · test: typing, deleting, a selection replaced, undo and redo grouped by word
- [x] T3 — `workspace.Save`, its errors, and the rule that only it writes · serves R2, R4 · test: `TestSavingWritesTheWholeFileAtOnce`, `TestSavingRefusesAFileChangedOnDisk`, `TestOnlyTheSaveWritesInARepository`
- [x] T4 — The place "editing", a second key for every command, and the help · serves R5 · test: `TestNoPlainKeyCommandWhileEditing`, `TestTheHelpListsEveryCommand`
- [x] T5 — The caret in the viewer: drawing it and the selection, clicks, typing, colouring again · serves R1 · test: a window test typing into a file
- [x] T6 — Saving from the window, the dot of a file not saved, asking before losing it, reading the changes again · serves R2, R6, R7 · test: `TestASavedFileShowsAsChanged`, `TestClosingAnEditedFileAsks`
- [x] T7 — The read-only mode: switch, key, flag, remembered · serves R3 · test: `TestReadOnlyModeTypesNothing`, `TestReadOnlyModeSavesNothing`
- [x] T8 — Documentation: the rule, the keys, the read-only mode · serves R8 · test: `TestOnlyReadOnlyCommandsRun` unchanged

## Result

Built on `feature/code-reader`, 6 November 2026, in seven commits from `418ca46` (T1, T2) to the
one that ticks T8.

| Id | Met | How |
|---|---|---|
| R1 | Yes | `internal/text` holds the file; the viewer paints the caret, the selection and the line it is on; arrows (Shift selects, ⌥ by words, ⌘ to the ends), Home, End, the page keys, a click, a drag and a double click; ⌘Z ⌘⇧Z ⌘X ⌘C ⌘V ⌘A. The file is coloured again 300 ms after typing stops. |
| R2 | Yes | `workspace.Save`: a temporary file renamed over the original, its permissions kept, a link followed to the file it points at; refused when the file's hash differs from the one the edits started from, and the window asks: keep mine, take theirs, cancel. |
| R3 | Yes | The lock in the title bar, `l` or ⌘⇧L, `-read-only`; remembered. No caret, nothing typed, pasted or saved, and the status bar says "read-only". |
| R4 | Yes | `TestOnlyTheSaveWritesInARepository`. |
| R5 | Yes | `TestNoPlainKeyCommandWhileEditing`; every command has a key with ⌘ (or an F-key, or Esc), and the help lists both. |
| R6 | Yes | Saving reads the project again: the tree, the counts and the diff show the change (`TestASavedFileShowsAsChanged`). |
| R7 | In part | ⌘Q, opening another file, opening or closing folders, and counting only commits ask to save, not save or cancel; a dot after the name marks edits not saved. **The window's close button, and ⌘Q when macOS's menu takes it first, still close without asking**: go-renderer leaves GLFW's default menu bar on and gives the program no way to refuse a close. That needs a change in go-renderer (or go-ui) first. |
| R8 | Yes | `internal/git` is unchanged: no command that writes. |

What changed from the plan:

- **No `text/errors.go`.** The buffer keeps every position inside the text instead of failing, so
  `text.position-out-of-range` had nothing to name. A new code, `workspace.outside-the-project`,
  refuses a path that leads out of its project.
- **⌘⇧N / ⌘⇧M** for the next and previous changed file, not ⌘N / ⌘M: macOS's window menu takes
  ⌘M to minimize.
- **A file ending in a line break shows the empty line after it**, numbered, as Zed does: the caret
  can go there.
- **Line endings are kept line by line**, so a file mixing "\n" and "\r\n" is saved as it was
  read; a new line break takes the ending most lines have.
- **The help groups the modes' keys apart, wraps its words, and scrolls** when the window is short.
- **The read-only mode has a plain key, `l`,** besides ⌘⇧L: where nothing is edited, letters run
  commands.

A review of the change found ten problems, all fixed with a test each except the close button (R7
above): a save after reading the project again overwrote a change on disk without asking; counting
only commits, opening or closing folders dropped edits; saving in the review mode went back to the
diff; Enter at the start of a line doubled its indent; a link was replaced by a file; mixed line
endings were all turned into "\r\n"; past 1,000 undo steps, undoing everything looked unchanged;
and a save from the question that met a change on disk forgot what was asked for.

Left for later: asking before the window closes (go-renderer); deleting by Backspace is undone line
by line, not word by word.
