# What Coddy does

It has two modes, chosen on the rail at the far left (or with `⌘1`, `⌘2`, `⌘⇧0`):

- **Files** (the default) is for reading code: every file of every project in a tree, the changed
  ones named in colour (green added, amber modified, red deleted), and a file opens whole with its
  changes marked beside the line numbers, as Zed marks them: a bar for lines added or changed, a
  wedge where lines were removed. A Markdown file opens formatted, as it reads: headings, lists
  and tasks, quotes, coloured code, tables and links. The eye in the bar at the top turns that off, to
  read its text, its changes, or to edit it.
- **Review** is for going through a development: only the changed files, each with the lines it
  adds and removes, opened as diffs. `⌘⇧V` marks the file as viewed and opens the next one not viewed;
  each project shows how much of it was viewed, and the status bar the whole. A mark is kept by the
  file's text, so a file that changes again is shown as not viewed (the marks live in
  `~/Library/Application Support/coddy/reviewed.json`, never in the repositories). With no
  file open, the review mode shows an overview of the projects: branch against base, commits,
  files, lines, progress.

- **Opens several repositories at once.** Give it repositories, or folders of repositories side by
  side (a workspace of twenty projects), on the command line or with `⌘O`, in the system's own folder chooser. The folder
  chosen with `⌘O` becomes the workspace, in place of the one open. Repositories are found up to
  three folders down. A folder with loose files beside its repositories is shown as one tree, each
  repository in its place with its branch and changes; a folder of repositories alone lists just
  them; a folder with none is a plain folder (every file, no change). The folders are remembered
  for the next start.
- **Shows what the development changed in each project**: added, modified, deleted and renamed
  files, measured from where the branch left its base (`git merge-base`), so what landed on the base
  since is not counted. The base is each project's `origin/HEAD`, or the first of `origin/main`,
  `origin/master`, `main`, `master`, `develop`; `⌘⇧B` (or `-base`) chooses another branch, tag or
  commit for one project or for all of them. Changes not committed yet count too, unless `⌘⇧W` (or
  `-committed`) leaves them out.
- **Reads a changed file as a diff**, both sides interleaved with their line numbers, whole at first
  (`⌘U` or the *Fold* button folds the unchanged lines away, Enter or a click opens one fold), with
  arrows to the previous and next change (`F7` and `⇧F7`), and a map of the changes along the scroll bar. Inside a replaced line the words that changed are washed stronger. `⌘D`
  switches to the whole file.
- **Only the changes**, when you ask: `⌘⇧G` or the button over the sidebar leaves the
  projects a development did not touch out of the sidebar, the finder and the language servers,
  and the *Files* mode lists only the changed files, with its folders left as you opened them.
  Every project and file is shown at first; the choice is remembered, and `-all` shows everything
  whatever was chosen.
- **Git's status can be hidden** (`…` → *Hide git status*, remembered): the names in the colour of
  how they changed, the `A` and `M` letters, the folders' dots and the counts of changed files
  leave the sidebar while you only read, and so do the file's status and lines changed in its
  title, and the marks in its gutter and along its scroll bar, formatted or not. The Review mode
  and every diff keep them: there they are what you read.
- **A filter over the sidebar** (`⌥F`) lists only the files whose path holds each of its words,
  folders open, and the projects it names, whole; Enter takes the keyboard to the first file found.
- **One bar at the top, for what is in front**: the file open, named by its path with how it
  changed, or the workspace while none is open; then, on the right, the search (`⇧⇧`) and the
  buttons of the mode on. The review's (the changes' arrows, folding, the diff or the whole file, viewed, the
  changes counted and the base) are there only in the Review mode; the others have the lock.
- **A folder chosen in the sidebar** names itself in the bar, and asks in the file's place for one
  of its files, or to find one with `⇧⇧`.
- **What is the review's stays in the review**: the Viewed button, `⌘⇧V` and how much was viewed
  in the status bar exist only in the Review mode.
- **The sidebar folds away** (`⌘⇧E`, or the panel button at the foot of the rail), and the file takes
  its room; it comes back when the keyboard goes to it (`Tab`, `⌥F`, `Esc`), and stays folded the
  next time if it was left so.
- **Every other file stays readable**: the *Files* mode lists them all, and the finder finds them.
- **Enter a project** to see only its tree, from its top (Enter on its row, `⌘E`, or the → that
  shows on its row under the pointer); `Esc` goes back to every project.
- **One button folds or opens everything** at the top of the sidebar: *Collapse all* while
  something is open, *Expand all* once nothing is. Outside, it folds or opens the projects;
  inside one, every folder of its tree.
- **Java and Kotlin projects shown as IntelliJ shows them**: Gradle and Maven modules marked,
  source roots blue (`src/main/java`, `kotlin`…), test roots green, resources apart, the packages
  inside a root joined with dots (`com.example.lighthouse.lamp`), and classes named without their
  extension with a badge for their kind (C class, I interface, E enum, @ annotation, R record, O
  Kotlin object), read from the files as they come into sight.
- **Icons by file type** in the tree and the finder, tinted with the code's own colours: Lucide's,
  built in, or any of [Zed's icon themes](https://zed.dev/docs/extensions/icon-themes), drawn in
  their own colours. Those installed in Zed are listed by themselves; others go in the reader's
  `icon-themes/` folder (see [Get started](2-get-started.md)). A file takes the icon of its whole
  name, then of its longest extension (`a.spec.ts`: `spec.ts`, then `ts`), as Zed matches it; with
  an icon theme, folders go by name, and Java and Kotlin classes keep their badges.
- **Syntax colouring** for Java, Kotlin, Vue, SQL, Gradle, Groovy, XML, YAML, Markdown and the
  rest of chroma's languages; text that is not UTF-8 is read as Latin-1.
- **Edits a file opened whole**: a caret, a selection (Shift, a drag, a double click), undo by
  word, cut, copy and paste. The file is coloured again a moment after you stop typing. `⌘S`
  writes the whole file at once, and refuses to overwrite one changed on disk since it was read: it
  asks whether to keep yours or take theirs. A dot after the name marks edits not saved, and
  opening another file or quitting asks first. A saved file shows as changed at once. The diff
  stays for reading.
- **Follows names with language servers**: `gopls` for Go, `jdtls` for Java and `kotlin-lsp` for
  Kotlin (JetBrains' preview, `brew install JetBrains/utils/kotlin-lsp`), each brought by an
  official plugin (`plugins/go`, `java`, `kotlin`; without them installed Coddy runs no server), found on
  the `PATH`, or where `go install` and Homebrew put commands (`~/go/bin`, `/opt/homebrew/bin`),
  and started the first time a file of their language opens. `⌘B` or `⌘`-click goes
  to where a name is declared, even in another project, and `⌘[` comes back; on the declaration
  itself `⌘B` shows its uses instead, as IntelliJ does, and `⌥F7` opens the uses mode on every
  use of a name (below); resting the pointer on a name shows its
  type and documentation. A server not installed is named, with the command that installs it, and
  its card in the servers panel has an *Install* button when that command's program (`go`,
  `brew`) is on the computer: it installs in the background, and the server starts once it is done.
  Every plugin says so of the programs it requires (`pandoc`, `gh`, `git`): Settings › Plugins
  lists those missing, with *Install*, or the command to run in a terminal when it needs `sudo`.
  One `jdtls` reads every Java project of the window, and one `kotlin-lsp` every Kotlin one, so
  names cross repositories; a name another repository gives as a jar opens in that repository's
  source when it is open. While a server reads the projects (`kotlin-lsp` imports each Gradle
  build, minutes the first time) a question waits for it. A bolt at the right of the status bar
  says how names are followed: crossed out and quiet while no server answers and only the text
  is searched, in the accent with a ring of how far the servers are while they read the projects,
  green once they are ready, with a warning's dot when one cannot run. Beside it, the first server
  at work says what it does in a few words (*Kotlin · importing 9/16 · ~1 min*): the time left is
  told by how long it took the time before, kept in its data folder. A click on it lists each
  server and its state, and *Details…* opens the servers panel: a card for each, with its state, a
  bar of how far it is, how long it took last time, its Gradle daemons running or released, the
  projects it leaves out with why, and its projects and last messages under *Show details*. A
  project the servers leave out is told three ways: *N left out* in the status bar, in the warning
  colour, which opens the panel; a crossed-out bolt on its row in the sidebar, whose note says why
  and that its uses are found by their text; and the panel's list. The status bar's
  left shows, as icons, the files changed, the projects with changes, and in the Review mode how much was viewed. Servers
  start once the projects are read, and read only the projects in sight.
- **A mode for the uses of a name** (`⌥F7`, or `⌘3` once a search was made): the sidebar lists only
  the files that use it, by project, the name's own first, then those with changes, each with its
  count. A file opens with its uses marked and the lines away from them folded, still editable
  (`⌘D` shows it whole). `F3` and `⇧F3` go from use to use across files, `⌥V` ticks one as seen, and
  the header counts them; `⌥R` searches again. While a server still imports the projects, a place
  whose file imports the name, or its package, shows at once as a probable use, counted apart
  (`2+1?`), until the server confirms it or not. Over the viewer, three views (`⌥1` `⌥2` `⌥3`):
  - the list of uses, in the files;
  - the graph of the name: the name in the middle, the projects that use it on either side, each
    file with its count. A click opens the file at its use; a `⌘`-click asks that file's server
    for the method or class the use is in, and lays out beside it the files that use that;
  - the graph of the repositories: each project, an arrow to each it depends on, read from its
    Gradle and Maven build files as text (nothing is run). Changed projects are edged, those that
    use the name lit; a dependency declared in a way not read (a version catalog, a coordinate
    made in code) is counted under it.

  The graphs zoom with the wheel, move with a drag, and `⌥0` fits them in again.
- **Servers that leave nothing behind**:
  - Indexes go in the reader's own folder, or in the one `-server-data` names (another disk; the
    choice is remembered). `kotlin-lsp` keeps one index per set of projects, in its data folder's
    `indexes/` (it would keep them in `~/Library/Caches/JetBrains` otherwise, where nothing removes
    them): one index for every set hung it when started with fewer projects than the index knew.
    It reads every Kotlin project of the window, whatever the filter shows, importing them once.
    Once it has imported a project with Gradle the reader keeps the project's model (kotlin-lsp's
    `workspace.json`, in `models/`) with a fingerprint of its build files; the next start reads the
    project from that model, through a folder standing in for it, with no Gradle at all, until a
    build file changes (`settings.gradle*`, `build.gradle*`, `gradle.properties`,
    `libs.versions.toml`, the wrapper's version). The servers panel says how many were read each way. `jdtls` too reads every Java project of the window from its start: it drops
    from its workspace the projects it was not started with, and would import each again with
    Gradle as a search of uses reached it. The reader keeps the index used last (`-keep-indexes` keeps more), and
    does not start it on a disk with less than 10 GB free, saying why.
  - `jdtls` writes nothing in the repositories: its `.project`, `.classpath` and `.settings/` go in
    its data folder, and it builds nothing into `bin/` or `target/`.
  - `kotlin-lsp` imports with a Gradle home of its own, in its data folder, holding only its
    daemons and their settings: its caches, Gradle versions and JDKs are yours, linked, so
    nothing is downloaded or made twice. Its Gradle daemons live through the whole import, and
    are stopped once the server is ready and quiet for 20 seconds, and with the server, never
    touching yours.
  - A server that imports Gradle builds is not given a build it cannot import, which would fail on
    every start: one needing Gradle 4 or older (it runs on Java 8 to 10 only), or an Android app
    when the Android SDK is not installed. Text search still reads them.
  - `kotlin-lsp` answers as soon as it starts and begins importing seconds later, answering with
    nothing meanwhile: the reader takes it as ready only once that first work is done.
  - Each Java server takes at most 2 GB of memory (`-server-memory` changes it).
  - A server nobody asks anything for 15 minutes rests, giving its memory back; following a name
    or asking its uses starts it again, from its index. A server still importing never rests, and
    is not restarted for the projects in sight: a cut import makes the next one start cold. The servers panel shows what each takes,
    memory and index, and stops one.
- **What a server writes is not your change**: `kotlin-lsp`'s Gradle import writes `.gradle/` and
  `build/`, as a build does, and `jdtls` wrote `.project` and `.classpath` before it was told not
  to. The reader notes what each project held before a server started in it, and a file
  that appears or changes afterwards, which you did not save in the reader, leaves the review, the
  counts and the progress. A dashed badge on the project, `kotlin-lsp · 4`, lists those files to open
  them, and the reader remembers them after a restart. Edits a server asks to make
  (`workspace/applyEdit`) are refused.
- **A read-only mode**: the lock in the title bar (in the Files and Uses modes), `⌘⇧L` or `-read-only` turn editing off, and
  the choice is remembered.
- **Writes a file only when you save it; git only reads.** Language servers may write their own
  files, shown apart as above. It only runs `git cat-file`, `diff`,
  `ls-files`, `merge-base`, `rev-list`, `rev-parse` and `symbolic-ref` (any other command is
  refused by `editor/internal/git/run.go`), without the optional locks, the file system monitor or
  external diff programs: no commit, no push, no checkout. It never fetches, so a remote branch is
  as fresh as the last `git fetch` you ran.

## Design

The interface follows what review tools have settled on, and keeps colour for meaning:

- **Colours are IntelliJ's Dark in the dark, and Zed's One Light in the light**
  ([one.json](https://github.com/zed-industries/zed/blob/main/assets/themes/one/one.json)), for
  the window and the code alike: neutral greys, the editor a step apart from the panels around
  it, green, amber and red only for added, changed and removed, blue only for where you are.
  Bubble 4 gives the shapes, spacing and type.
- **The settings are a screen of their own**, in place of the sidebar and the file, opened from
  the gear in the bar over the file (`⌘,`; `?` opens them on the keys) and closed with Esc. Their
  sections are listed on the left: *Appearance*, each theme as a small window painted in it and
  each icon theme as a card of a few of its icons; *Memory*, how much memory the files you
  open may keep (a slider from 32 to 512 MB that says about how many files that is, 64 to start with, remembered), so going back to one is
  instant; *Keys*, every command with its keys drawn as caps; and *Plugins*, a grid of cards, one for each
  plugin: its icon (its own, or its initial on a colour of its own), name, version, author, two
  lines of what it does, its state and a switch, and the programs it misses with how to install
  them. Chips above choose a kind (all, languages, version control, review, actions, modes), each
  with how many there are; turning on or off a plugin that waits for the next start says so. A
  plugin with settings has a gear on its card that opens them (a text, a secret kept in the
  system's keyring, a switch or a choice each), saved as they change.
- **Plugins** (an experiment) add language servers, and actions on the file open in the "…" menu,
  such as exporting it to PDF. Each is a folder in Coddy's own folder, on until you turn it off;
  git is a plugin built into Coddy, which can be turned off too (folders are then not read as git);
  [writing a plugin](5-guides/5-writing-a-plugin.md) says how. A plugin can bring pull requests,
  as `plugins/github` does with `gh` and `plugins/azure-devops` with Azure DevOps' REST API (a
  token in its settings, kept in the system's keyring): *Review a pull request…* in the "…" menu lists a project's,
  and choosing one reviews its changes against its base, its comments beside their lines.
- **Themes**, chosen in the settings (the gear in the bar, or `⌘,`) and remembered: Zed's One Dark and One Light,
  IntelliJ's Dark and Light, Visual Studio Code's Dark Modern (its colours, from
  [microsoft/vscode](https://github.com/microsoft/vscode), MIT), and [Dracula](https://draculatheme.com/)'s Dracula Solid
  (from [dracula/zed](https://github.com/dracula/zed), MIT), and juanmnl's
  [Mr Pink](https://github.com/juanmnl/vs-mr-pink) (MIT) come with it. Your own are Zed's theme files: drop any
  of them (a theme family, `.json`) in `~/Library/Application Support/coddy/themes/` and it
  is listed the next time the reader starts; one with the name of a theme of the reader takes its
  place. The reader takes the colours it paints with from the file's `style` (`text`,
  `editor.background`, `created`, `modified`, `deleted`…) and its `syntax`, and makes up those a
  file leaves out from the text and the editor's ground.
- **Changes marked in the gutter** while reading a whole file, and a map of them along the scroll
  bar, as [Zed's git integration](https://zed.dev/docs/git.html) does.
- **Words that changed inside a line** are marked over the line's wash, as most diff tools do; the
  `+` and `−` signs stay, so the diff reads without telling red from green
  ([Review Board's colour-blind mode](https://reviews.reviewboard.org/r/10981/)).
- **Viewed files and progress**, as GitHub's *Viewed* box and Reviewable: a large development is
  reviewed across days, and the reader remembers where you were.
- **Room to read**: rows of 28 pixels in the tree, code on lines of 22, the path to a file quiet
  and its name plain, icon buttons with their key in the tooltip.

## Not done yet

- No search inside a file.
- No side-by-side diff; the sides are interleaved.
- Long lines are not wrapped: scroll sideways (Shift+wheel, ←/→ in the editor).
- One typeface for code at a time: the system's by default, or the one `-code-font` names (a `.ttf`;
  collections such as `Menlo.ttc` are not read).
- Language servers only follow names: no rename, formatting, completion or diagnostics yet, and
  not on the old side of a diff, which is not on disk.
