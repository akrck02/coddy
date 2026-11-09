# 6. Uses across projects, the projects in sight, and what servers are doing

> Status: in progress · Author's approval: proposal ☑ plan ☑

## Why

Trying proposals 4 and 5 on the big workspace (44 repositories, 22 with changes) showed three things:

- **Uses stop at the repository.** `⇧F12` on a Kotlin class of `dal.mariadb` listed 1 use, its own
  declaration. The class is also used in a Java file of `harbour.api.order`. Each repository of the big workspace is
  a Gradle build of its own, and the others see it as a jar, so no server links that use to the
  source: kotlin-lsp does not read Java files of other builds, and jdtls sees the jar's class. A
  review needs those uses: who calls what changed.
- **The list was taken while the server was still reading.** The answer had one location, so it was
  not "empty" and was not asked again (proposal 5 asks again only answers with nothing).
- **Servers read every project, and say only "reading the projects…".** Opened with every project,
  kotlin-lsp imported 44 Gradle builds with over a dozen Gradle daemons, minutes long, and the status
  bar said the same words throughout. The "only the changed files" button opened every folder
  instead of leaving out the projects with no changes, and those were read by the servers anyway.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | Uses of a name include those in other projects in sight, in any language a server reads: a Java file using a Kotlin class of another repository, through its jar. | A window test with two projects whose servers answer a jar location (fake server), and a test on the big workspace by hand. |
| R2 | A list of uses taken while a server is still reading says so, and fills in by itself once the servers are done. | A window test with the fake server busy. |
| R3 | Filtering to the changed files leaves out, in the sidebar and the finder, the projects with no changes; it does not open folders. | A sidebar test. |
| R4 | Servers read the projects in sight only. When that set changes, a server that follows workspace folders is told; one that does not (kotlin-lsp) is started again on the new set. | Window tests with the fake server, with and without workspace folders. |
| R5 | The status bar says, per server, what it is doing: the phase (starting, importing, indexing, ready), the project, how far (a count or a percentage, as the server says), and for how long. | Client tests on `$/progress`, `language/status` and `intellij/importLog`; a window test of the status text. |
| R6 | Clicking a server in the status bar opens a panel: each project it reads with its state, the time each took, and the server's last messages and errors. | A window test. |
| R7 | A question asked while a server reads says what it waits for ("waiting for kotlin-lsp: indexing 45 %") instead of nothing. | A window test. |

## Out of scope

- Uses of a name in projects not open, or hidden by the filter.
- Joining the big workspace's builds into one (a Gradle composite build): it would let one server see every source,
  but it changes the repositories, and they are not the reader's to change.
- Rename, diagnostics, completion.

## Design

### Uses across projects (R1, R2)

The server of the file gives the uses it knows. For the other projects in sight, the reader:

1. Searches their files for the name as a whole word, with the finder's content search (already
   fast on the big workspace). Only files a server reads (`.java`, `.kt`, `.go`) count.
2. Checks each hit: asks that file's server for the definition at the hit. A hit is a use when the
   answer is the same declaration: the same file and line, or a class in a jar that `fromJar` maps
   to it (proposal 4, 5). Hits whose server answers elsewhere (another class with the same name)
   are dropped.
3. Lists the uses grouped by project, the file's own project first. The list's footer says
   "checking 40 of 120 places in 6 projects" while it checks, and the list grows as it goes.

At most 300 hits are checked, the nearest first (same project, then projects with changes); the
footer says when some were left out. A hit in a project whose server is still reading waits for it,
and says so (R2). A list asked while the file's own server reads is asked again when it is done,
whatever it answered.

### The projects in sight (R3, R4)

- The changed-files button (`g`) and the projects button (`h`) become one filter, "only changes":
  projects with no changes leave the sidebar and the finder, and the trees list only changed files,
  opened as they are now when a file is revealed, not all at once.
- One-per-window servers get the projects in sight as workspace folders. A project that comes into
  sight is added (`didChangeWorkspaceFolders`, as now); one that leaves is removed. A server that
  does not announce `workspace.workspaceFolders` (kotlin-lsp) is stopped and started again on the
  new set, a moment after the last change, so a few clicks restart it once. Its index stays in its
  data folder, so a restart reads only what it has not read.

### What servers are doing (R5, R6, R7)

The client keeps, per server, a `Progress`: the phase, the project, the count or percentage, when
it started, and the last 50 messages. It reads:

- `$/progress` begin, report and end (title, message, percentage), from any server;
- `language/status` (jdtls: "Importing Gradle project(s)", "x %", `ServiceReady`);
- `intellij/importLog` (kotlin-lsp: "Importing with Gradle: <project>"), and `window/logMessage`
  errors.

The status bar shows one short line per server that is not ready, e.g.
`kotlin-lsp · importing harbour.api.order (7 of 22) · 3:12` or `jdtls · indexing 45 % · 0:40`,
and `kotlin-lsp ✓` once ready. Clicking it opens the servers panel (R6). A request waiting on a
server toasts what it waits for (R7), and the toast goes when the answer comes.

## Tasks

- [x] T1 One "only changes" filter: projects without changes leave the sidebar and the finder; no folder opened by it. (R3)
- [x] T2 Servers read the projects in sight: add and remove folders; restart a server without workspace folders. (R4)
- [x] T3 `Progress` in the client from `$/progress`, `language/status`, `intellij/importLog`, logs; the status bar line. (R5, R7)
- [ ] T4 The servers panel. (R6)
- [ ] T5 Uses across projects: candidates from the content search, checked by definition, grouped by project, with the footer. (R1)
- [ ] T6 Lists taken while busy say so and fill in. (R2)
- [ ] T7 Try it on the big workspace: the `TideTableStore` case, timings and memory with the filter on and off; README and Result.
