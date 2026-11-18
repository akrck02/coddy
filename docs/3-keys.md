# Keys

The keys follow IntelliJ's macOS keymap where the reader does what IntelliJ does (`⌘B`, `⌥F7`,
`⌘[` / `⌘]`, `F7` in a diff, `⌘⇧O`, `⌘⇧F`, `⌘⇧-` / `⌘⇧+`); the single letters are the reader's own.
While you edit a file, letters type: commands then take the key with `⌘` (or `⌃`), shown second
below, an F-key or `Esc`. `Esc` gives the keyboard back to the sidebar, where the letters work.

| Key | |
|---|---|
| `⌘F` | Filter the projects and files the sidebar lists; Enter goes to them |
| `⌘⇧O`, `⌘O`, `⌘P` (or `Shift+P`) | Find a file by its name, in any project; changed files first |
| `⌘⇧F` (or `/`) | Find a text in the files of every project; `Tab` in the finder switches between names and content |
| `n` / `m` (`⌘⇧N` / `⌘⇧M`) | Next / previous changed file, across every project |
| `F7` / `⇧F7`, `]` / `[` | Next / previous change in the file (the diff, or the marked lines) |
| `1` / `2` / `3` (`⌘1` / `⌘2` / `⌘3`; `c` or `⌘⇧0` switches) | Files mode / Review mode / Uses mode |
| `Enter` on a project, `e` (`⌘E`), or its → button | Enter the project: the sidebar lists only its tree, and `n`/`m`/`v` stay in it |
| `Esc` / `Backspace` | Back to every project |
| `v` (`⌘⇧V`) | Mark the file as viewed (or not); marking it opens the next file not viewed |
| `d` (`⌘D`) | Diff, or the whole file |
| `f` (`⌘U`) | Fold the unchanged lines, or show them all again (a diff opens whole) |
| `⌘⇧-` / `⌘⇧+` | Fold every unchanged line / show them all |
| `Tab` (`⌃Tab`) | Move between the sidebar and the editor |
| `↑ ↓ ← →`, `PgUp PgDn`, `Home End` | Move in the sidebar (←/→ close and open folders), or scroll the file |
| `j` / `k`, `Space` | Scroll a line, a page |
| `Enter` | Open a file or a folder (sidebar), open a fold (editor) |
| `g`, `h` (`⌘⇧G`) | Only the changes, projects and files, or everything (sidebar, finder and servers) |
| `b` (`⌘⇧B`) | Compare against another branch, tag or commit |
| `w` (`⌘⇧W`) | Count the changes not committed yet, or the commits only |
| `r` (`⌘R`) | Read every project again |
| `o` (`⌘⌥O`) | Open a repository or a folder of repositories |
| `⌘+` / `⌘-`, `⌘0`, or `⌘` and the wheel over the code | Larger / smaller code, or its first size |
| `y` (`⌘⇧C`) | Copy the open file's path |
| `t` (`⌘⇧T`) | Light or dark |
| `l` (`⌘⇧L`) | Read-only, or editing |
| `⌘B` (`F12`), or `⌘`-click | Go to where the name at the caret (or clicked) is declared; on the declaration, its uses |
| `⌥F7` (`⇧F12`) | Every use of the name at the caret (or clicked), in the uses mode |
| `⌘[` / `⌘]` (`⌘⌥←` / `⌘⌥→`) | Back to where you were before following a name, and forward again |
| `?` (`F1`) | These keys |

In the uses mode, its keys come first:

| Key | |
|---|---|
| `n` / `⇧N` | Next / previous use, across files |
| `v` | Tick the use as seen, and go to the next not seen |
| `r` | Search the uses again |
| `⇧1` / `⇧2` / `⇧3` | The list of uses / the graph of the name / the graph of the repositories |
| `0` | Fit the graph in the window |
| click / `⌘`-click on a file of the graph | Open it at its use / who uses the method or class that use is in |

Editing a file:

| Key | |
|---|---|
| `⌘S` | Save |
| `⌘Z` / `⌘⇧Z` | Undo / redo |
| `⌘X` `⌘C` `⌘V`, `⌘A` | Cut, copy, paste, select all |
| Arrows (`Shift` selects), `⌥←` `⌥→` | Move by a character, a word |
| `⌘←` `⌘→` (or `Home` `End`), `⌘↑` `⌘↓` | The start or end of the line, of the file |
| `⌥Backspace` | Delete the word before the caret |
| `Esc` | Stop editing: the sidebar takes the keyboard |
