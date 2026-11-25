# Keys

Every key holds `⌘` (`Ctrl` off a Mac) or `⌥` (`Alt`), or is a function key or a key that moves. A letter
alone would be typed into a field by mistake, and on another keyboard it is another letter, so
Coddy uses none. Where Coddy does what IntelliJ does, it takes IntelliJ's macOS key (`⌘B`, `⌥F7`,
`⌘[` / `⌘]`, `F7`, `⌘⇧F`, `⇧⇧`).

`F1` opens the keys screen: every action by topic, with its icon and its keys, and a field that
searches them.

## Profiles: your own keys

Coddy's own keys are a profile, **Coddy**, that does not change. To change keys:

1. On the keys screen, press **New profile** and give it a name. It starts as a copy of the
   profile chosen, and is chosen at once.
2. Click a key of an action, then press the new keys. `Esc` stops without changing it.
3. Keys that would type (a letter, or a letter with `Shift`) are refused. Keys another action
   uses there already show a warning: **Use it** gives them to this action, and the other one
   loses them.
4. The arrow beside a changed action gives it Coddy's keys again.

Each profile is a small JSON file in `keymaps/` of Coddy's folder (`~/.config/coddy/keymaps` on
Linux). It lists only the actions it changes, by their id, one chord for each of Coddy's keys:

```json
{
  "name": "Harbour",
  "keys": {
    "open-folder": ["Alt+O"],
    "step-change": ["Alt+Down", ""]
  }
}
```

An empty chord takes that key away. The modifiers are `Command` (or `Control`), `Alt` and `Shift`;
Coddy starts again with the profile chosen last.

## Coddy's own keys

| Key | |
|---|---|
| `⌘O` | Open a repository, a folder of repositories or any folder, in the system's own folder chooser |
| `⇧⇧` (Shift twice) | Find a file by its name, in any project; changed files first |
| `⌘F` | Find a text in the files of the project you are in: the one entered, chosen in the sidebar, or of the file open |
| `⌘⇧F` | Find a text in the files of every project; `Tab` in the finder switches to names |
| `⌥F` | Filter the projects and files the sidebar lists; Enter goes to them |
| `⌘E`, or a project's → button | Enter the project: the sidebar lists only its tree |
| `Esc` / `Backspace` | Back to every project |
| `⌘⇧N` / `⌘⇧M` | Next / previous changed file, across every project |
| `F7` / `⇧F7` | Next / previous change in the file |
| `⌘D` | Diff, or the whole file |
| `⌘U`, `⌘⇧-` / `⌘⇧+` | Fold the unchanged lines, or show them; fold or show them all |
| `⌘⇧G` | Only the changes, projects and files, or everything |
| `⌘⇧B` | Compare against another branch, tag or commit |
| `⌘⇧W` | Count the changes not committed yet, or the commits only |
| `⌘R` | Read every project again |
| `⌘B` (`F12`), or `⌘`-click | Go to the declaration of the name at the caret; on the declaration, its uses |
| `⌥F7` (`⇧F12`) | Every use of the name at the caret, in the uses mode |
| `⌘[` / `⌘]` (`⌘⌥←` / `⌘⌥→`) | Back to where you were before following a name, and forward again |
| `⌘1` … `⌘9`, `⌘⇧0` | A mode by its place on the rail; the next mode |
| `⌘⇧V` | In the Review mode: mark the file as viewed, and open the next one not viewed |
| `⌘⇧E` | Hide the sidebar, the file taking its room, or show it again |
| `Tab` (`⌃Tab`) | Move between the sidebar and the file |
| `⌘⇧L` | Read-only, or editing |
| `⌘+` / `⌘-`, `⌘0` | Larger / smaller code, or its first size |
| `⌘⇧C` | Copy the open file's path |
| `F11` | The whole screen, or a window again |
| `⌘,`, `F1` | The settings; `F1` opens them on the keys. `Esc` closes them |
| `↑ ↓ ← →`, `PgUp PgDn`, `Home End`, `Space`, `Enter` | Move in the sidebar, or scroll the file |

In the uses mode:

| Key | |
|---|---|
| `F3` / `⇧F3` | Next / previous use, across files |
| `⌥V` | Tick the use as seen, and go to the next not seen |
| `⌥R` | Search the uses again |
| `⌥1` / `⌥2` / `⌥3` | The list of uses / the graph of the name / the graph of the repositories |
| `⌥0` | Fit the graph in the window |

Editing a file, where letters type:

| Key | |
|---|---|
| `⌘S` | Save |
| `⌘Z` / `⌘⇧Z` | Undo / redo |
| `⌘X` `⌘C` `⌘V`, `⌘A` | Cut, copy, paste, select all |
| Arrows (`Shift` selects), `⌥←` `⌥→` | Move by a character, a word |
| `⌘←` `⌘→` (or `Home` `End`), `⌘↑` `⌘↓` | The start or end of the line, of the file |
| `⌥Backspace` | Delete the word before the caret |
| `Esc` | Stop editing: the sidebar takes the keyboard |
