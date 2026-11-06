# 4. Language servers: definition, references and hover

> Status: building · Author's approval: proposal ☑ plan ☑

## Why

Reviewing a change that spans many repositories means following names across them: where a class
is declared, who calls a method, what a type is. Today the reader has only the finder's text search
for that. Language servers answer it exactly, for Go with `gopls` and for Java with `jdtls`, which
matters most for the big workspace.

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
| Java, Kotlin | `jdtls` | window | every Java project as a workspace folder; its data folder in the reader's own folder |

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
- [ ] T5 — References and hover · serves R2, R3 · test: `TestReferencesAreListed`, `TestHoverShowsTheServersText`
- [ ] T6 — What servers write: the list before, the comparison, the record on disk · serves R6 · test: `TestFilesWrittenByAServerAreNotChanges`
- [ ] T7 — The badge and the list of a server's files · serves R7 · test: `TestFilesWrittenByAServerAreListed`
- [ ] T8 — Refusing `applyEdit` · serves R8 · test: `TestApplyEditIsRefused`
- [ ] T9 — Trying it on the real servers: `gopls` on this repository, `jdtls` on the big workspace · serves R1–R7 · test: an integration test that skips when the server is not installed, and a note of what `jdtls` wrote in the big workspace

## Result

Not built yet.
