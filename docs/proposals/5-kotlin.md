# 5. Kotlin: a language server of its own

> Status: done · Author's approval: skipped by the author ("build it directly"); this file is the record

## Why

About a third of the big workspace is Kotlin, and `jdtls` reads Java only ([4](4-language-servers.md), Result).
Without a Kotlin server, `F12`, `⇧F12` and hover say nothing in those files.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | Kotlin files have a server: definition, references and hover work as for Java. | `TestKotlinLspFollowsANameAcrossFiles`, on the real server, skipped when it is not installed. |
| R2 | A question asked while the server is still reading the projects waits for it, instead of answering "nothing found". | `TestARequestAnsweredWithNothingWhileTheServerWorksIsAskedAgain`. |
| R3 | A Kotlin name in another repository's jar opens in that repository's source, when it is open: a type, an `object`, a function, an extension, a file's functions. | `TestAKotlinDeclarationInAJarOpensItsSource`. |
| R4 | The rules of proposal 4 hold: what the server writes is shown apart, edits it asks for are refused, a missing server is named. | The tests of proposal 4, unchanged. |

## Design

- **Which server.** JetBrains' `kotlin-lsp` (a preview, built on IntelliJ), installed with
  `brew install JetBrains/utils/kotlin-lsp`, run as `kotlin-lsp --stdio --system-path <folder>`. The
  older community server (`fwcd/kotlin-language-server`) is no longer maintained. One server per
  window, like `jdtls`, with every Kotlin project as a workspace folder; its index in the reader's
  own folder.
- **Waiting for it.** It sends no ready message, as `jdtls` does: it reports its work with
  `$/progress` ("Importing", then "Indexing…") and answers every request with nothing meanwhile.
  The client follows that work (a begin adds a token, an end removes it). A server with work
  running is not ready (the status bar says it reads the projects), and a request it answers with
  nothing while busy is asked again once it is idle, three times at most. This holds for any server
  that reports progress, not only this one.
- **Jars.** It gives a name declared in a dependency as `jar:///<jar>!/org/lighthouse/lamp/Lamp.class`.
  `ClassInJar` now reads both this and `jdtls`'s `jdt://`, and gives the class without extension,
  the outer one of an inner class. The reader looks for `Lamp.java`, `Lamp.kt`, `Light.kt` for the
  class `LightKt` Kotlin makes of a file's functions, and else any Kotlin file of the package's
  folder that declares the class. In the source, the line is a type declaration (`object` and
  `typealias` added), else a Kotlin `fun`, `val` or `var` of that name (an extension's included),
  else what proposal 4 did.

## Tasks

- [x] T1 Follow `$/progress`; a busy server is not ready; ask again an answer with nothing.
- [x] T2 The fake server reports work (`FAKE_LSP_BUSY`) and answers nothing meanwhile.
- [x] T3 `jar:` locations, Kotlin sources and declarations.
- [x] T4 `kotlin-lsp` in the table; a test on the real server.
- [x] T5 Tried on the big workspace.

## Result

What the server did on a repository of the big workspace (a Gradle build in Kotlin and Java, its sibling
repositories as SNAPSHOT jars):

- **Cold**, it imported the Gradle build for about 3 min 40 s, then indexed; every request
  meanwhile was answered with `[]`. **Warm** (its index kept in the system path), a definition was
  answered in about 10 s, with no indexing reported.
- A definition of a class of a sibling repository came as a `jar:` location of the `.class`, mapped
  as above when that repository is open.
- Hover is Markdown with a `java` fence and a four-backtick `kotlin` fence, and a link to the
  package with no address: the card already drops fences and keeps a link's words.
- **It wrote nothing new in the repository of the big workspace**, whose `.gradle/` and `build/` were already there
  and ignored by git. Its index goes in its system path. On a fresh project (the real-server test)
  Gradle's import writes `.gradle/` and `build/` in it, as any build does; where git does not ignore
  them they show apart as the server's files, as in proposal 4.

Left as they are:

- It does not say it follows workspace folders (`workspace.workspaceFolders` is absent from its
  capabilities), so a Kotlin project opened after it started may not be read until the window is
  opened again.
- It is a preview build: version 263.7084.0-EAP has an EAP licence that ends on 13 November 2026,
  though `kotlin-lsp license status` also says the build needs none. If it stops starting after
  that date, `brew upgrade jetbrains/utils/kotlin-lsp` gets a newer build (a cask of the same name now exists, so the bare name means the cask).
- The bundle ships without its EULA text (`--show-eula` fails with "EULA.txt is missing").
