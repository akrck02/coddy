# 9. Servers that import faster

> Status: in progress (block 1 built; block 2 being built) · Author's approval: proposal ☑ plan ☑ (block 1) · block 2 plan ☑

## Why

With more than ten projects open the Java and Kotlin servers became unusable: minutes before a
name could be followed, and again after each rest. On the big workspace (45 repositories, 16 Gradle versions
from 4.10.3 to 9.6.0, about 1,500 Kotlin and 4,100 Java files) a measure on 18 November 2026 found
where the time goes.

## What was found

Marked **seen** (measured, in logs or code) or **not tried**.

1. **jdtls imports Gradle builds one after another** (seen): Buildship syncs one root at a time,
   about 16 s each. 411 of 501 s of a cold start were Gradle sync; 80 to 110 s warm.
2. **kotlin-lsp imports every Gradle build on every start** (seen), all at once: 15 to 20 daemons,
   90 to 177 s, even with its index warm. Cold, it then indexes about 1.1 million library files,
   about 6 more minutes.
3. **The daemons were made to die early** (seen): the 2-minute idle time of proposal 8 (R5), and
   the flag `com.jetbrains.ls.imports.gradle.daemon.no.idle.timeout`, which in fact sets the idle
   time to 1 ms. A daemon gone between two builds is a daemon started again: 5 to 10 s each.
4. **A cut import is lost** (seen): stopping kotlin-lsp in the middle of an import (resting it,
   or restarting it when the projects in sight change) leaves an index it does not trust, and the
   next start imports cold.
5. **Five repositories fail on every start** (seen): two builds of Gradle 4, which runs on Java 8
   to 10 only, and three Android apps without the Android SDK. Each failure is a daemon and its
   time, every start.
6. **kotlin-lsp can import from a description instead of Gradle** (seen in its files, not tried):
   the command `exportWorkspace` writes a `workspace.json`, `initializationOptions.buildTools`
   tells it to read one, and `libexec/bin/warmup.py` does both. Version 271 also brings an
   `lsp-router`. A description kept per repository, made again only when its build files change,
   would skip Gradle on most starts.
7. **jdtls has a shared index** (seen in its docs, not tried): `-Djdt.core.sharedIndexLocation`
   indexes the JDK and libraries once for every workspace.
8. **The machine** (seen): `~/.gradle` is on a USB disk (HFS+, Spotlight indexing it); 16 GB of
   memory for two servers and up to 20 daemons; the internal disk 87 % full.

## What

Three blocks: the reader's own waste first, then the description cache, then what only the person
can do.

| Id | Requirement | Check |
|---|---|---|
| R1 | Daemons of a server's own Gradle home live 30 minutes idle, not 2, and the 1 ms flag is gone. Once the server is ready and quiet for 20 s, the reader stops them: they are not kept for hours, and not killed between two builds of one import. | `lsp` tests: the idle time written; ready and quiet a while is done importing. |
| R2 | A server still importing never rests, and is never restarted for the projects in sight; it is once ready. | Window tests with a fake server that never says it is ready. |
| R3 | A server that imports Gradle builds is not given a project it cannot import (Gradle below 5; Android without an SDK). The reader remembers why, for the person to see; text search still reads it. | `lsp` tests of each rule; a window test leaves the project out. |
| R4 | kotlin-lsp imports from a `workspace.json` kept per repository, made again when its build files change. | Block 2: to plan, after trying `lsp-router` and `warmup.py`. |
| R5 | The person's part: Spotlight off on the external disk, or Gradle's caches on the internal one. | Block 3: by hand. |
| R6 | The person sees what the servers are doing and what was left out, and why. | After the blocks: to design. |

## Tasks

- [x] T1 Daemons: 30 minutes idle, stopped once the import is done. (R1)
- [x] T2 No rest and no restart in the middle of an import. (R2)
- [x] T3 Builds that cannot be imported are left out, with their reason. (R3)
- [x] T4 Measure block 1 on the big workspace: time to ready, daemons during and after. (R1–R3)
- [x] T5 Block 2: try `exportWorkspace`, `buildTools` and `lsp-router`; plan R4.
- [ ] T6 Block 3 with the person. (R5)
- [x] T7 Design how the servers' work and the projects left out are shown, and build it. (R6)

## Notes while building

- **T1**: "done importing" is the server ready (jdtls's `ServiceReady`, kotlin-lsp's first answer)
  with no work in progress for 20 s, checked every 5 s. A new import (work in progress again)
  starts the wait over, and its daemons are stopped once it is done too.
- **T3**: the Gradle version comes from the wrapper's properties; Android from `com.android.` in a
  `build.gradle(.kts)` at the top or one folder down; the SDK from `ANDROID_HOME`,
  `ANDROID_SDK_ROOT` or `sdk.dir` in `local.properties`. Only servers that import Gradle builds
  (`OwnGradle`) leave them out; gopls and the others see every project.
- **T4**: kotlin-lsp 271 answers `initialize` at once and begins importing 10 to 24 s later,
  answering with nothing meanwhile: the reader took it as ready, and the real-server test failed
  (on the commit before block 1 too). It is now ready once its first work ends (`ReadyAfterWork`),
  or 45 s after it started if it begins none. The daemons' reaper also counts each project it
  begins importing as work, not only what it reports with `$/progress`.

## Result of block 1 (T4, on the big workspace, 19 November 2026)

30 Java projects, 16 Kotlin ones; 5 left out of both (2 of Gradle 4, 3 Android without the SDK).

| | Before | Cold | Warm |
|---|---|---|---|
| jdtls ready | 501 s cold, 80 to 110 s warm | 305 s | 13 s |
| kotlin-lsp import | 90 to 177 s, every start | 72 s | 260 s |
| kotlin-lsp ready (import and indexing) | about 8 min cold | 459 s | 324 s |
| Daemons during the import | 15 to 20, each started again and again | 16 | 16 |
| Daemons a minute after ready | kept up to 3 hours, or killed between builds | 0 | 0 |

- jdtls gains the most: its Gradle sync reuses daemons that live through the whole import, and
  the projects left out no longer fail each start.
- kotlin-lsp still imports every build on every start, warm or not: the warm import took longer
  than the cold one, with the external disk shared with jdtls's run just before. That is block 2's
  work (R4). The first start with the 5 builds left out made a new index, being a new set of
  projects: indexed cold once.

## T7: what the person is told

Designed on a canvas with the author and built as drawn, at three levels:

- **At a glance, the status bar**: beside the servers' bolt, the first server at work in a few
  words with about how long it has left (*Kotlin · importing 9/16 · ~1 min*), and *N left out* in
  the warning colour when the servers leave projects out; a click on it opens the servers panel.
  The time left comes from how long the server took the time before, from its start to ready with
  no work running, kept in its data folder (`reading-took`). A question waiting for a server says
  the same in its toast.
- **Where it matters, the sidebar**: a crossed-out bolt on the row of a project left out; resting
  the pointer on it says why and that its uses are found by their text; a click opens the panel.
- **The detail, the servers panel**: each card says how long the server took last time, its
  Gradle daemons running, or those released once it was done and when, and *Left out of this
  server* with each project and why.

## T5: block 2, tried (19 November 2026)

Tried by hand with a small client (`warmup.py`'s way) on the 19 Kotlin repositories of the big workspace that
are not Android apps, with the reader's Gradle home.

**What kotlin-lsp 271 has** (seen in its jars):

- An importer `json` (`JsonDriver`): it reads `workspace.json` in a workspace folder, and is
  chosen for a folder by `initializationOptions.buildTools` (`{folder URI: "json"}`).
- A command `exportWorkspace <folder>`: it writes the model it imported (modules, libraries, SDKs,
  Kotlin and Java settings) to `<folder>/workspace.json`, every path absolute. For the 19
  repositories: 114 modules, 1,331 libraries, 5 MB.
- So the file can live in the reader's data folder, in a folder standing in for the repositories:
  given that folder, kotlin-lsp reads the real files by their absolute paths, and a definition in a
  library answers as with Gradle. Nothing is written in a repository.
- `intellij/ready-for-test`, a notification once the index is built and saved: a truer "ready"
  than the end of its work (`ReadyAfterWork`), not sent when there is nothing to index.
- `lsp-router` keeps one server alive across editors, by workspace. Not tried: it keeps a JVM of
  2 GB when the reader is closed.

**Found on the way, fixed**: the index keeps the model with paths by the place of each folder
among those the server started with (`$WORKSPACE_7$/src/main`). Started with fewer folders than
the model names, kotlin-lsp cannot read it and hangs, answering nothing (30 minutes, 374 errors
in its log). One index for every set of projects, built this morning, did that: back to one per
set, now in `indexes/<hash of the folders>`, the last kept; `rocks/` is removed.

| 19 repositories | Gradle (today) | `workspace.json` |
|---|---|---|
| Ready, index built | 452 s | **12.6 s** |
| Ready, cold | 254 s to 459 s (16 repositories) | 102 s |
| Gradle daemons | 16 | 0 |
| A definition in a library | answers | answers the same |

## Plan of block 2 (R4), approved 20 November 2026

- **R4.1** For each repository a server imports with Gradle, the reader keeps its part of the
  model, `workspace.json`, in the server's data folder, with a fingerprint of its build files
  (`settings.gradle*`, `build.gradle*` at any depth, `gradle.properties`,
  `gradle/libs.versions.toml`, `gradle/wrapper/gradle-wrapper.properties`).
- **R4.2** kotlin-lsp starts with one stand-in folder per repository whose model is current
  (`buildTools: json`), and the real folder, with Gradle, for each one without a model or whose
  fingerprint changed.
- **R4.3** Once a start with Gradle is ready, the reader asks `exportWorkspace`, splits the model
  by repository (the modules under its folder and the libraries and SDKs they use) and keeps each
  part with its fingerprint.
- **R4.4** A build file changed while the server runs marks that repository's model old: the next
  start imports it with Gradle (kotlin-lsp already re-imports it then, as now).
- **R4.5** The servers panel says how each repository was read: from its model (and since when)
  or with Gradle.

Tasks:

- [ ] T8 Fingerprint of a repository's build files, and the model kept with it. (R4.1)
- [ ] T9 Stand-in folders and `buildTools` at start; real folders for the old ones. (R4.2)
- [ ] T10 Export once ready, split by repository. (R4.3, R4.4)
- [ ] T11 Say it in the panel. (R4.5)
- [ ] T12 Measure on the big workspace: first start (Gradle), second (models), one build file changed.

Risks: the importer and the command are not documented and may change with kotlin-lsp; the
reader checks the export's shape and falls back to Gradle when a model cannot be read. Each new set
of stand-in folders is a new index (cold, about 100 s): the folders are named by repository, so
a repository changing between Gradle and its model does change the set once.
