# 8. Language servers without waste

> Status: in progress · Author's approval: proposal ☑ plan ☑

## Why

On 9 November 2026 a morning of trying proposals 4 to 6 on the big workspace left, on the Mac's own disk:

- **9.5 GB of kotlin-lsp indexes**, two of them, one per set of projects it was given (4 to 5 GB
  each for 12 to 44 repositories). It had already filled the disk once (`RocksDBException … No
  space left` in its log). The reader now keeps the two used last ([671baec]).
- **15 Gradle daemons**, 13 Gradle versions, alive for hours after kotlin-lsp stopped: its
  Gradle import always goes through a daemon, and a daemon waits 3 hours before it exits.
- **Files jdtls wrote in repositories**: `.project`, `.classpath`, `.settings/`, and `bin/` with a
  copy of the resources. The reader shows them apart (proposal 4), but they need not be written.

A reviewer opens a window, reads, closes it, and opens another tomorrow. Servers should cost what
that needs, and leave nothing behind.

## What was found

Marked **seen** (in docs, code, logs or on disk) or **not tried**.

1. **kotlin-lsp follows workspace folders**, contrary to what proposal 5 said: its `initialize`
   answer has `workspaceFolders.supported: true` (seen, in its own log). The reader already tells
   it of projects that come and go, without restarting it.
2. **Its index is named by the set of folders it starts with** (seen: the same set reused the same
   index; another set made another). Folders added later go into the same index (not tried). So
   the folders it *starts* with decide whether yesterday's index is reused.
3. **Options it has** (seen, `kotlin-lsp --help`): `--multi-client` with `--socket` and
   `--port-file` serve several windows from one server; `--idle-timeout` stops a server with no
   client; `IJ_JAVA_OPTIONS` sets its memory (`-Xmx`). None excludes tests or libraries; since
   263.4702 its Gradle import downloads library sources too, which costs disk.
4. **Gradle daemons** (seen, Gradle docs): `org.gradle.daemon.idletimeout` in the Gradle user
   home's `gradle.properties` wins over a repository's own. A server given its own Gradle user home
   (`GRADLE_USER_HOME`) has daemons of its own, which the reader can time out and stop without
   touching those of the person's IDE; `GRADLE_RO_DEP_CACHE` lets that home reuse the person's
   downloaded dependencies (not tried with kotlin-lsp).
5. **jdtls can be told not to write** (seen, its settings): `java.import.generatesMetadataFilesAtProjectRoot: false`
   keeps `.project`/`.classpath`/`.settings` in its data folder; `java.autobuild.enabled: false`
   stops `bin/`; `java.configuration.updateBuildConfiguration: disabled` stops re-imports on each
   edit. `-Xmx` goes on its command line.
6. Joining the repositories in one generated Gradle composite build, or precomputed SCIP indexes,
   could give one server every source, but 13 Gradle versions make the first fragile and SCIP's
   Kotlin is partial (not tried). Text search checked by each file's server (proposal 6) stays.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | A server for the whole window starts with a set of projects that stays the same from one day to the next for the same review: the projects with changes and those they depend on (proposal 7's build reading), so the same index is used again. Others are added while it runs, never by restarting it. | A window test with the fake server: the start set; one added later does not restart it. |
| R2 | Before starting kotlin-lsp, the reader checks the free space of the data folder's disk: under 10 GB it does not start it and says why, with the size of its old indexes and `-server-data` to move them. | A test with a fake free space. |
| R3 | Indexes: one kept (the one in use) by default; a setting keeps more. | `lsp` tests, as now. |
| R4 | Servers stop after 15 minutes without a question and start again on the next, from their index (seconds, warm). | A window test with a short wait. |
| R5 | kotlin-lsp runs with its own Gradle user home in the reader's folder, daemons timing out after 2 minutes and reusing the person's downloaded dependencies; when it stops, the reader stops the daemons of that home. No daemon of the reader outlives the window by more than 2 minutes. | A test with the fake server that starts a fake daemon; checked by hand on the big workspace. |
| R6 | jdtls writes nothing in the repositories: the settings of finding 5. | The real-server test asserts no file is new. |
| R7 | Memory: kotlin-lsp and jdtls get `-Xmx` (2 GB each by default, a setting changes it). | Checked by hand. |
| R8 | The servers panel (proposal 6) shows each server's memory, its index's size on disk, and a button that stops it. | A window test. |

## Out of scope

- One server shared by several windows (`--multi-client`): worth it once two windows are usual.
- The composite build and SCIP of finding 6.

## Tasks

- [x] T1 jdtls's settings and `-Xmx`; the real-server test asserts it writes nothing. (R6, R7)
- [x] T2 kotlin-lsp's own Gradle user home, daemons timed out and stopped. (R5, R7)
- [x] T3 Free space checked; one index kept. (R2, R3)
- [x] T4 Idle servers stop and start again. (R4)
- [ ] T5 The start set from changes and their dependencies (needs proposal 7's T5). (R1) —
  waits for proposal 7.
- [x] T6 Memory, index size and a stop button in the panel. (R8)
- [ ] T7 Try it on the big workspace for a day of reviews: disk and processes before and after; Result.

[671baec]: the commit that removes old indexes

## Notes while building

- **T1**: the setting `java.import.generatesMetadataFilesAtProjectRoot` alone did not stop jdtls:
  it reads it as a JVM property (`-Djava.import.generatesMetadataFilesAtProjectRoot=false`) before
  it imports. The real-server test now imports a Maven project and asserts nothing new is in it;
  with the old settings it found `.project`, `.classpath`, `.settings/` and `target/classes/`.
  Projects where jdtls wrote them already keep them: they are listed as the server's (proposal 4).
- **T2**: kotlin-lsp honours `GRADLE_USER_HOME` (seen: its daemons log in the reader's Gradle home).
  The wrapper folder is a link to the person's, so no Gradle version is downloaded twice. A daemon
  is known by the log it writes, `daemon/<version>/daemon-<process>.out.log`, and stopped only when
  that process is a Gradle daemon. Its first import with the new home reindexes once, as jar paths
  change.
- **T3**: `-keep-indexes` and `-server-memory` change the table's values from the command line.
- **T4**: pointing at a name does not wake a resting server (hovering would start 2 GB by
  accident); following a name and asking its uses do.

## Result (T7, first try on the big workspace, 9 November 2026)

44 repositories, 23 with changes; `~/.gradle` had just moved to an external disk (a link).

| | Before | During | After the window closed |
|---|---|---|---|
| Free disk | 60 GB | | 62 GB |
| kotlin-lsp data | 4.2 GB, one index | 1.2 GB, the same index | 1.2 GB |
| Gradle daemons | 0 | 0 | 0 |
| Java servers' memory | — | jdtls 0.9 GB; with kotlin-lsp 2.5 GB | 0 |
| Files new in the repositories | — | — | none (`git status --ignored` the same in all 44) |

- kotlin-lsp started only when a search of uses reached Kotlin files, five minutes in. It started
  with the same set of projects as the day before, so it **reused its index**: its workspace model
  came back from cache and it answered in 12 seconds, with no Gradle import and so no daemon. The
  index shrank from 4.2 to 1.1 GB on that start (RocksDB compacting it, presumably; not checked).
- Not seen yet: a cold import with the reader's own Gradle home on the big workspace, and a server resting after
  15 minutes; the window was closed first. The real-server test covers the first on a small build.
- Seen in passing (proposal 6): a word in a Javadoc comment, not a `{@link}`, counted as a use, as
  jdtls answers its definition. Places in comments should be left out of the uses checked.
