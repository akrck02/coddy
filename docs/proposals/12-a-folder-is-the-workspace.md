# 12. A folder is the workspace, repositories and loose files together

> Status: done · Author's approval: proposal ☑ plan ☑

## Why

A folder opened with `⌘O` is the workspace, as in VS Code or Zed. Today Coddy shows only the
repositories found inside it, so everything else in the folder disappears: loose files, folders
with no repository, an archive beside them. Someone who opens `Downloads` sees eleven
repositories, but not the PDF, the zip or the folders that are not repositories.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | The sidebar shows the folder opened as one tree: its loose files and folders, and each repository in its place inside it | test `TestAFolderShowsItsRepositoriesInsideItsTree` |
| R2 | A repository inside the tree still reads as a repository: its branch icon, its branch, its changes, its row and keys | test `TestARepositoryInsideTheTreeKeepsItsChanges` |
| R3 | A file outside every repository is listed once, and a repository's files only under it | test `TestALooseFileIsListedOnceAndARepositorysFilesOnlyUnderIt` |
| R4 | When the tree cannot be shown (the review lists only changed files, projects without changes are hidden, a project is entered), the repositories are listed flat, as today | test `TestTheReviewListsTheRepositoriesFlat` |
| R5 | A folder that is a repository, or holds no repository, opens as today | tests `TestAFolderWithNoRepositoryOpensAsAPlainFolder` (`session`, `window`) and `TestRepositoriesSomeFoldersDownAreFound` |

## Out of scope

- More than one folder as a tree at once: folders given on the command line keep today's
  behaviour, each one discovered on its own.
- Changes in loose files: they have no version control, so they never show as changed.
- Language servers on the loose files: they run on the repositories, as today.

## Design

**Session.** When a folder opened holds repositories but is not one, it also becomes a plain
project of its own, the *root*. The root reads every file on disk except those inside its
repositories (`workspace.FolderControl` gets the paths to leave out, `LeftOut`). Each repository
remembers the root it was found in: a new `Project.Root` field, the root's path, empty for a
project that is not inside one; its place is its path from the root.

**Workspace tree.** The root's tree gets an empty folder for each repository's place, so that a
folder holding only repositories still has a row.

**Sidebar.** `addFolder`, while it lists the root, puts a repository's project row, and its
content when expanded, where its folder would be, one level deeper. `addEveryProject` skips a
repository whose root is listed. When the root is not listed (hidden, entered, or the listing
chooses files), the repositories are listed flat, as today. Rows keep their project index, so
selection, keys, filters and modes need no change.

**Not taken:** a separate plain project beside the repositories, with only the loose files. It is
three files of change, but it shows the same folder twice in two places, which is what this
proposal fixes. A single tree type for every project with nested version controls was also
rejected: it would rewrite the sidebar, the finder and the review for one case.

## Tasks

- [x] T1 — `NoVersionControl` leaves out the repositories' folders; `discover` adds the root and
  sets `Inside` · `workspace/folder.go`, `workspace/project.go`, `session/discover.go` · R3, R5 ·
  test: `TestALooseFileIsListedOnceAndARepositorysFilesOnlyUnderIt`
- [x] T2 — the root's tree has a folder for each repository's place · `workspace/tree.go` · R1
- [x] T3 — the sidebar nests the repositories in the root's tree, flat when the root is not listed
  · `sidebar/rows.go` · R1, R2, R4 · tests: `TestAFolderShowsItsRepositoriesInsideItsTree`,
  `TestARepositoryInsideTheTreeKeepsItsChanges`, `TestTheReviewListsTheRepositoriesFlat`
- [x] T4 — docs: `1-features.md`, `4-how-it-is-made.md` · R1

## Changed while building

- The field is `Project.Root`, the root's path, not `Inside`: the place follows from the two paths,
  and the sidebar needs to find the root anyway.
- A folder that holds nothing but repositories gets no root: a level above them would add only a
  row. Many of the window's tests, and a folder like `~/Code`, are this case, and keep today's list.
- A plain folder is left out of a list of changes only: it never has one.
- Revealing a file of a nested repository (`⇧⇧`, a definition) opens the root down to it.

## Result

Built: R1 to R5, each with its test, plus `TestAFolderOfRepositoriesAloneListsOnlyThem` and
`TestAFileOfARepositoryInsideTheTreeIsRevealed`. Checked read-only on a real downloads folder: 11
repositories nested, 211 loose files listed. Not looked at on screen yet: the rows' indentation
and icons need a person's eye.
