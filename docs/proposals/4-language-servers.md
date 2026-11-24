# 4. Language servers: definition, references and hover

> Status: done, Kotlin left out (see Result) · Author's approval: proposal ☑ plan ☑

## Why

Reviewing a change that spans many repositories means following names across them: where a class
is declared, who calls a method, what a type is. Today the reader has only the finder's text search
for that. Language servers answer it exactly, for Go with `gopls` and for Java with `jdtls`, which
matters most for the large workspaces of many Gradle repositories it was tried on (the big workspace from here on).

The reader used to have to prove that a server never writes in a repository, and `jdtls` does write
(`.project`, `.classpath`, `.settings/`, and Gradle's `.gradle/` and `build/`). The author decided
on 6 November 2026 that servers may write, on two conditions: what they write is shown, and it is
never counted as the person's change in the review.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | From a name in a file, go to where it is declared, even in another project. | `TestDefinitionOpensTheDeclaration`, against a fake server (T2). |
| R2 | List every use of a name, and open any of them. | `TestReferencesAreListed`. |
| R3 | Resting the pointer on a name shows its type and documentation. | `TestHoverShowsTheServersText`. |
| R4 | Going somewhere can be undone: back to where the person was. | `TestBackReturnsToThePlaceBefore`. |
| R5 | A server starts only when a file of its language is opened, and stops with the window. | `TestAServerStartsOnFirstUse`, `TestServersStopWithTheWindow`. |
| R6 | What a server writes in a repository is not counted as a change of the person: not in the review, the counts or the progress. | `TestFilesWrittenByAServerAreNotChanges`. |
| R7 | What a server writes is shown, per project, and can be opened. | `TestFilesWrittenByAServerAreListed`. |
| R8 | A server never changes a file the person is editing: edits it asks for are refused in this proposal. | `TestApplyEditIsRefused`. |
| R9 | Without the server installed, the reader works as today and says what to install. | `TestAMissingServerIsNamed`. |
| R10 | Every error carries a code. | `lsp/errors_test.go`. |

## Out of scope

- Rename, format, code actions and completion: they edit files through the server
  (`workspace/applyEdit`) and get their own proposal once this one is used. Until then the client
  answers every `applyEdit` with "not applied" (R8).
- Diagnostics (errors and warnings in the code).
- Following names on the old side of a diff: that text is not on disk, and servers only read disk.
- Servers other than `gopls` and `jdtls`; adding one is a line in a table.
- Installing servers: the reader finds them on the `PATH`.

## Design

### Packages

- `internal/lsp`: the client. JSON-RPC 2.0 over a server's standard input and output, written here
  (about 500 lines) to add no dependency. It sends `initialize`, `initialized`, `textDocument/didOpen`,
  `didChange`, `didClose`, `definition`, `references`, `hover`, `shutdown` and `exit`, and answers
  the server's own requests (`workspace/configuration`, `window/workDoneProgress/create`,
  `workspace/applyEdit` refused). It imports no interface package; a rules test checks it.
- `internal/lsp` also keeps the table of servers: a language, the command, and its settings.
- The window owns the running servers and connects them to the viewer, as it does with background
  work.

### Which servers, and how many

| Language | Server | One per | Settings |
|---|---|---|---|
| Go | `gopls` | module root | none |
| Java (Kotlin: see Result) | `jdtls` | window | every Java project as a workspace folder; its data folder in the reader's own folder |

One `jdtls` holds every Java project open, because a definition in the big workspace usually crosses repositories
and each `jdtls` takes hundreds of megabytes. A server starts the first time a file of its language
is opened (R5), and it is told about each file opened, with the text as the editor has it, edits
included (that is why proposal 3 comes first).

### In the viewer

| Action | Keys |
|---|---|
| Go to the declaration | F12, or ⌘ and a click on the name |
| Every use | ⇧F12: the results in a panel like the finder's, by project and file |
| Type and documentation | the pointer resting on a name for half a second |
| Back, forward | ⌃-, ⌃⇧- |

While the server is still reading the projects (`jdtls` can take minutes on the big workspace), the status bar
says so, and a request waits for it instead of failing.

### What servers write

The reader cannot tell who wrote a file by looking at it, so it notes what was there before:

1. **Before a server starts in a project**, the reader keeps the list of files git sees as changed
   or new and not ignored, with a hash of each.
2. **When the project is read again**, a file that is new or different since then, and that the
   editor did not save (`workspace.Save` keeps the list of what it saved, with hashes), was written
   by the server.
3. **The list is kept** in the reader's own folder, by project, as the marks of files viewed are, so
   after a restart `.classpath` is still the server's and not a new file of the person.

Files git ignores (`build/`, `.gradle/` in most projects) are already left out of the changes and
stay so.

Those files leave the review (R6): no row in the review mode, not in "N changed files" or the bar
of progress. The tree marks the project with a quiet dashed badge, "jdtls · 4", which opens a list
of them (R7).

One case cannot be decided: a file the person changes outside the reader while a server runs looks
like the server's. It is listed with the server's files, marked "changed while jdtls ran", rather
than hidden.

### Errors

`lsp.server-not-found`, `lsp.server-stopped`, `lsp.request-failed`, `lsp.bad-message`, each tested.

## Tasks

- [x] T1 — `internal/lsp`: messages, the JSON-RPC connection and its errors · serves R10 · test: requests, replies and notifications over a pipe; a broken message
- [x] T2 — A fake server for the tests, a small Go program in `testdata` that answers from a table · serves R1–R3 · test: it is what T3–T5 use
- [x] T3 — Starting and stopping servers, the table of servers, a missing server · serves R5, R9 · test: `TestAServerStartsOnFirstUse`, `TestServersStopWithTheWindow`, `TestAMissingServerIsNamed`
- [x] T4 — Definition and back · serves R1, R4 · test: `TestDefinitionOpensTheDeclaration`, `TestBackReturnsToThePlaceBefore`
- [x] T5 — References and hover · serves R2, R3 · test: `TestReferencesAreListed`, `TestHoverShowsTheServersText`
- [x] T6 — What servers write: the list before, the comparison, the record on disk · serves R6 · test: `TestFilesWrittenByAServerAreNotChanges`
- [x] T7 — The badge and the list of a server's files · serves R7 · test: `TestFilesWrittenByAServerAreListed`
- [x] T8 — Refusing `applyEdit` · serves R8 · test: `TestApplyEditIsRefused`
- [x] T9 — Trying it on the real servers: `gopls` on this repository, `jdtls` on the big workspace · serves R1–R7 · test: an integration test that skips when the server is not installed, and a note of what `jdtls` wrote in the big workspace

## Result

T1–T8 are built and tested against the fake server. T9 ran on the real servers, gopls 0.24.0 and
jdtls 1.63.0: `TestGoplsFollowsANameInThisRepository` and `TestJdtlsFollowsANameAcrossFiles` pass
(they skip where a server is not installed), and jdtls was tried on two repositories of the big workspace, a web API
and the common library it uses.

What T9 found on the big workspace:

- **jdtls takes about 75 seconds** to read two repositories the first time, and 3 with its index
  kept. Until it says `ServiceReady` it answers nothing, so requests now wait for it and the status
  bar says "jdtls reading the projects…". The plan had promised this; the first build had not done it.
- **Another repository is a jar.** Each repository of the big workspace is its own Gradle build, and uses the others
  as published jars, so a name declared in the common library is found in its jar, not in its
  folder. jdtls is now asked for jar locations (`classFileContentsSupport`), and the reader opens
  the class's source when its repository is open in the window, at the line where the name is
  declared there (the jar's lines need not be the source's). A class whose repository is not open
  is named in a toast.
- **What jdtls wrote**: `.project`, `.classpath` and `.settings/` in the web API, which git does
  not ignore, and `bin/`, which it does; nothing in the library. The first three are what R6 and R7
  are for.
- **A third of the big workspace is Kotlin, and jdtls does not read Kotlin.** The table now gives jdtls Java only;
  Kotlin needs a server of its own, a proposal of its own: [5](5-kotlin.md).

| Id | Met | How |
|---|---|---|
| R1 | yes; in the big workspace through the jar's class | F12 on the caret or the place clicked, or ⌘-click; across projects with one server for the window |
| R2 | yes | ⇧F12 lists the uses in the finder's panel, the development's files first; typing narrows them |
| R3 | yes | half a second at rest on a name; the card closes when the pointer leaves |
| R4 | yes | ⌃- and ⌃⇧-, a hundred places back, kept by path on disk |
| R5 | yes | started on the first file of its language, stopped with the window; one that stops on its own starts again, three times at most |
| R6 | yes | below |
| R7 | yes | a dashed badge on the project, "jdtls · 4", opens the list in the finder's panel |
| R8 | yes | the client answers "not applied" |
| R9 | yes | a toast and the status bar name the server and the command that installs it |
| R10 | yes | `lsp.server-not-found`, `server-stopped`, `request-failed`, `bad-message`, and `workspace.record-not-kept` |

What changed from the plan:

- **Who wrote a file.** Only a file that differs from the last commit on disk, and that the reader
  did not save, can be a server's: one that changed since the snapshot taken before the server
  started, or one recorded as the server's that still holds the text it left (or while a server
  runs). A commit, a pull or another base never hides a change. The reader cannot tell a file the
  person made outside it from the server's, so the list says "new while jdtls ran: by it, or by you
  outside the reader" (or "changed while…"), rather than "written by".
- **The snapshot reads each project afresh** before the server starts, which delays the start by
  one reading of the projects (on the big workspace, of every Java project).
- **The finder's panel holds the uses and the server's files**, as a third scope, rather than a
  panel of its own.
- **Requests about the file tell its text first**, on the same goroutine, with versions that only
  grow: the server never answers about an older text.
- **Notifications are handled in order**, in the connection, after a test caught a didChange
  overtaking its didOpen.
- **F12 and ⇧F12 without a caret** (read-only, or in the diff) ask about the place clicked last.

The review of the build found nine problems, all fixed with a test where one could be written: a
commit or a pull while a server ran hid the person's changes; a recorded file stayed hidden
whatever it became; a missing server left a snapshot; a project where only a server wrote vanished
with its badge; a server that crashed stayed "running"; texts could reach the server out of order;
a file read again from disk was not told again; the trail kept project indices; and a jump
cancelled at the unsaved question left a trail entry and a column.
