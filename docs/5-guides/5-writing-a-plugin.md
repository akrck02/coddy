# Writing a plugin

This guide is for someone who wants Coddy to do something it does not, without changing Coddy:
read a new language, or do something with the file open, such as exporting it. A plugin is a
folder with a `plugin.json` that names a program, and sometimes the program itself. Plugins are an experiment: the protocol
may still change, and its version says when it does.

## The idea

A plugin adds to Coddy; it does not replace any of it. Every plugin is the same thing: a program
that Coddy runs and talks to, JSON-RPC messages over its standard input and output. What changes
is the language the program speaks, and the manifest says which in `speaks`:

| `speaks` | The program is… | What it can do is said by… |
|---|---|---|
| `"lsp"` | a language server written for any editor | the Language Server Protocol, which Coddy knows already |
| `"coddy"` | written for Coddy, with its sdk | the plugin's `contributes` |

Think of a language server as a plugin whose manifest Coddy ships with: what it can do (go to a
definition, find the uses of a name) is the protocol's own, so its plugin only says how to run
it. A program written for Coddy has no such standard behind it, so it says what it adds, and
Coddy draws it: an action is an item of the "…" menu, a version control reads folders with its
marker. Coddy reads that from the manifest, without starting the program, and starts it only
when there is work.

Git is the one exception: its code is compiled into Coddy, so it has no program at all. To the
rest of Coddy it looks the same as a version control plugin, since both answer the same
`codebase.VersionControl` questions. A plugin that crashes
cannot take the window with it; Coddy says so, with the error's code, and starts it again the
next time it is used.

## A plugin that speaks lsp

The smallest plugin is a language server someone else wrote. `plugins/python/plugin.json`
runs Pyright:

```json
{
  "name": "python",
  "version": "0.1.0",
  "description": "Reads Python with Pyright.",
  "command": ["pyright-langserver", "--stdio"],
  "speaks": "lsp",
  "requires": [{
    "program": "pyright-langserver",
    "install": { "linux": ["npm install -g pyright"], "darwin": ["npm install -g pyright"] }
  }],
  "contributes": {
    "languageServer": {
      "languages": ["python"],
      "rootMarkers": ["pyproject.toml"]
    }
  }
}
```

The `command` is a program on the PATH, and the server is named after it (`pyright-langserver`).
`languages` are the protocol's names for the languages it reads. A plugin that speaks lsp
contributes only its `languageServer`: an action or a mode would need a program that speaks
Coddy's protocol. Servers join at Coddy's start. Coddy has no servers of its own: Go, Java and
Kotlin are read by its official plugins, `plugins/go`, `plugins/java` and `plugins/kotlin`, which
are plugins like this one. When two plugins read one language, the first folder by name wins.

## What a plugin requires

Most plugins lean on a program someone else made: a language server, `pandoc` to print, `gh` to
read GitHub. `requires` lists them, each with how to install it on each system, named as Go
names them (`linux`, `darwin`, `windows`):

```json
"requires": [{
  "program": "pandoc",
  "install": {
    "linux": ["sudo dnf install pandoc", "sudo apt install pandoc", "brew install pandoc"],
    "darwin": ["brew install pandoc"]
  },
  "environment": []
}]
```

Coddy looks for each program when it reads the plugins, not in every frame. Settings › Plugins
shows the missing ones on their plugin's card. Why a list per system? Linux has many package
managers, so Coddy offers the first command whose installer (`dnf`, `apt`, `brew`) is on the
computer. It runs that command itself, with *Install*, unless the command starts with `sudo`.
A window has no terminal to type a password in, so that one is shown for the person to run.
`environment` is added while the install runs (gopls wants `GOTOOLCHAIN=auto`). A language
server's own program goes here too: the servers' panel offers the same install.

### Tuning a heavier server

Most servers need only the fields above. A server on the Java virtual machine that imports
Gradle builds needs more, and the official Java and Kotlin plugins show each field at work:

| Field | Use it when the server… |
|---|---|
| `dataFlag` | keeps its data in a folder it is given on the command line (`-data` for jdtls) |
| `onePerWindow` | should read every project of the window as one workspace, not one per root |
| `readsAll` | keeps one index per set of projects, so it should read all of them from the start |
| `initializationOptions`, `foldersInOptions` | documents options at initialize, or reads its folders there |
| `settings` | asks `workspace/configuration` for sections of settings |
| `ready.status`, `ready.afterWork` | says when it has read the projects, or is ready once its first work ends |
| `memory.most`, with `flag` or `variable` | runs on a JVM whose memory Coddy caps (`-server-memory` changes it) |
| `index.folder`, `keep`, `option`, `retired`, `needsFreeGigabytes` | builds one index per set of projects, which Coddy keeps few of |
| `gradle.keepModels`, `ownHome`, `after` | imports Gradle builds: keep their models, give it its own daemons, wait for another import |

Why kotlin-lsp needs its index fields: it builds one index per set of folders it starts with,
and since version 271 it keeps them in the person's caches unless told otherwise, one more each
time the set changes. One index for every set does not work either: it records projects by
their place among the folders it started with, and started with fewer it hangs. So Coddy gives
it a folder per set (`option`), keeps the last (`keep`), and refuses to start it on a nearly
full disk (`needsFreeGigabytes`).

## How it shows in the settings

Settings › Plugins draws every plugin as a card: its icon, its `name` and `version`, its
`author`, two lines of its `description`, the programs it misses, and a switch. Two fields are
there only for that card:

```json
{
  "name": "lighthouse",
  "author": "Someone",
  "icon": "icon.svg"
}
```

`author` is who made it; Coddy's own plugins say `"Coddy"`. `icon` is one of
[Lucide's](https://lucide.dev/icons) names (`"coffee"`), drawn in a colour of the plugin's own, or
an SVG or PNG file of the plugin's folder (`"icon.svg"`), drawn as it is in a 40-pixel square
with rounded corners. The file must stay inside the folder: a path that climbs out with `..`, or
an absolute one, makes the manifest fail with `plugins.bad-manifest`. Without an icon, Coddy
draws the plugin's initial on a colour picked from its name, so a plugin keeps the same colour on
every computer. `plugins/go` has none, so it shows a G.

## A plugin that speaks coddy

A program written for Coddy. `command` runs it: its first word is a file of the plugin's folder,
or else a program on the PATH, and it runs inside the folder.

```json
{
  "name": "export-pdf",
  "command": ["export-pdf"],
  "speaks": "coddy",
  "contributes": { "actions": [{ "id": "export", "title": "Export to PDF" }] }
}
```

Coddy starts the program the first time an action runs, not before, and talks to it like this:

| Coddy sends | It means | The plugin answers |
|---|---|---|
| `initialize` `{protocolVersion, settings?}` | "this is the protocol I speak, and what you were set to" | `{protocolVersion}`, its own |
| `action/run` `{action, file}` | "run this action on this file" | `{note?, created?}` |

A `note` is shown for a few seconds. `created` is the absolute path of a file the action made;
Coddy says so, and the menu offers to open it. An error in the reply is shown as it is, so say
what went wrong in words a person understands. Each request has a time limit (two minutes for an
action); a plugin that misses it is stopped.

## Settings

Some plugins need to be told things only the person knows: the address of their server, which
pull requests they care about, a token. A plugin declares them in its manifest, and Coddy draws
them for it, so the plugin never builds a settings screen of its own.

```json
{
  "name": "github",
  "settings": [
    {
      "id": "pullRequests",
      "title": "Pull requests",
      "description": "Which open pull requests the review lists.",
      "type": "choice",
      "choices": ["Every open one", "Mine", "Asking for my review"],
      "default": "Every open one"
    }
  ]
}
```

| Field | What it is |
|---|---|
| `id` | the setting's name in the values the program is given; unique in the plugin |
| `title`, `description` | what the settings call it, and a line on what it is for |
| `type` | `"text"`, `"secret"` (a text shown masked), `"boolean"` or `"choice"` |
| `default` | its value until the person sets one: a string, or `true`/`false` for a boolean |
| `choices` | what a `"choice"` may be, in order; only for a choice |

A card whose plugin has settings shows a gear beside its power button, which opens them in a
sheet. Each change is saved as it is made: a switch or a choice at once, a text when its field
is left. A setting without a default is empty, off, or its first choice. A manifest with a
setting that has no `id`, an `id` twice, an unknown `type`, or a choice without `choices` fails
with `plugins.bad-manifest`.

The program gets the values in `initialize`, as `settings`: every setting by its `id`, with its
default when the person set none. That is the only place it reads them. When the person changes
one, Coddy stops the program, and its next use starts it with the new values; there is no other
message to follow. Plugins written before settings existed never see the field, so the protocol's
version stays 1.

The values are kept in `plugin-settings/<name>.json`, beside the `plugins/` folder, a file only
its owner can read (mode 0600). A plugin with settings must have a `name` that can name that
file. A secret never goes there: a file gets copied, backed up and synced, so a token in one
travels further than anyone meant. Coddy keeps each secret in the system's keyring instead, under
the service `coddy` and the account `<plugin>/<setting id>`: with `secret-tool` on Linux (GNOME
Keyring or KWallet, through libsecret), and in the login keychain on a Mac. A computer without
one keeps no secret: saving one fails with `keyring.not-available`, and the sheet says what to
install. There is no file to fall back to, on purpose. Windows has no keyring in Coddy yet
(a `TODO(akrck02)`).

The keyring is read when the plugin's program starts, not when Coddy reads the plugins: a locked
keyring may ask for its password, and a window should not wait for it to draw. That is also why
the sheet shows a secret's field empty after Coddy starts again: Coddy has not read it, and does
not need to until the program does.

## A plugin that reads another version control

Coddy reads git repositories by itself. Git is a built-in plugin: compiled into
Coddy, listed in Settings › Plugins, and on until turned off. A plugin can teach it another version control: it names
a *marker*, a file or folder every repository of it holds, and Coddy reads any folder opened that
has the marker (and no `.git`, which git keeps) by asking the plugin.

```json
{
  "name": "mercurial",
  "command": ["mercurial"],
  "contributes": { "versionControl": { "name": "Mercurial", "marker": ".hg" } }
}
```

Coddy asks only what it needs to list a development's changes and show a changed file's two
sides. Each request names the repository's `folder`; paths are relative to it, with forward
slashes. The questions are asked off the frame, as Coddy's git commands are, each within a
minute:

| Coddy sends | It means | The plugin answers |
|---|---|---|
| `versionControl/branch` `{folder}` | "what is checked out?" | its name |
| `versionControl/defaultBase` `{folder}` | "what do I compare against, if the person chose nothing?" | a revision, or `""` for none |
| `versionControl/files` `{folder}` | "which files are there?" | every file it knows, and the new ones it does not ignore |
| `versionControl/changes` `{folder, base, workingTree}` | "what changed since base?" | `[{status, path, oldPath?, untracked?}]` |
| `versionControl/read` `{folder, revision, path}` | "what did this file say then?" | its text |
| `versionControl/diff` `{folder, base, workingTree, path, oldPath?}` | "how does it differ?" | a unified diff, as `git diff` writes it |

`workingTree` says whether the files on disk count, or only the last commit; `"HEAD"` as a
revision always means the last commit, whatever the version control calls it. A `status` is `A`
added, `M` modified, `D` deleted or `R` renamed. Ask hg or svn for a diff with every line of the
file (`--unified 1000000`), since Coddy shows both sides whole. An error in a reply is shown on
the project, with its code; the other projects go on as before. Lines added and removed, the
marks of a file already reviewed and the commits ahead are git's alone for now.

## A plugin that brings pull requests

A *review source* tells Coddy which pull requests a project has, and what was said on their
lines. Coddy does the rest itself: it fetches the pull request from the repository's `origin`,
compares its head against its base, and shows the comments beside their lines.

```json
{
  "name": "github",
  "command": ["github"],
  "contributes": { "reviewSource": { "name": "GitHub" } }
}
```

| Coddy sends | It means | The plugin answers |
|---|---|---|
| `review/list` `{folder}` | "which pull requests are open?" | `[{id, title, author, baseRef, headRef}]` |
| `review/comments` `{folder, id}` | "what was said on its lines?" | `[{file, line, author, text}]` |

`baseRef` is the branch the pull request goes into (`main`); `headRef` is anything `git fetch
origin` takes that names its commits: GitHub keeps every pull request at `pull/N/head`, so a
fork's needs no remote of its own. A comment's `line` is a line of the file as the pull request
has it, from 1; `0` leaves it out. In Coddy, *Review a pull request…* in the "…" menu lists them
for the project of the file open (or the one selected); choosing one fetches it and turns on the
Review mode; choosing a base again (`⌘⇧B`) leaves it. Fetching is the only thing Coddy writes in a repository (objects and refs in `.git`):
the files and the branch checked out stay as they are, and nothing is sent to the remote.
`plugins/github` is the example, with `gh`; it only reads.

`plugins/azure-devops` is the other, for Azure DevOps, and it shows a plugin that talks to a
server itself, with `net/http`, instead of running a command. It reads the organization, project
and repository from the remote `origin` (any of the URL and SSH forms Azure gives), and asks the
REST API with a personal access token: its secret setting `token`, or else
`AZURE_DEVOPS_EXT_PAT`. For Azure DevOps Server on a company's own machine, its setting
`organizationUrl` is the collection's URL. Its errors say what to do (`azure-devops.no-token`,
`azure-devops.unauthorized`), since their words are the warning on its card. It only reads too.

## A plugin with a screen of its own

A plugin may add a mode to the rail, whose screen it draws itself with go-ui. Its manifest says
the mode's name and its icon (one of Lucide's names):

```json
"contributes": { "mode": { "name": "TODOs", "icon": "list-todo" } }
```

While the mode is on, its screen takes the viewer's place. Coddy asks the plugin for a frame with
`mode/frame` (the size, Coddy's theme, what the pointer did, and the folders of the projects
open), and the plugin answers with what it drew: a list of rectangles, texts and icons, and maybe
a line of a file to open. Coddy asks only when the pointer did something over the screen, or the
last frame asked for another; the rest of the time it paints the last frame again, so a screen
that stays still costs the plugin nothing, and a slow plugin shows its last picture.

In Go, the sdk's `screen` package does all of this: write the build function, as for any
go-ui interface, and give the screen's `Answer` to `Handlers.DrawMode`. It is a package apart, so only plugins that draw depend on go-ui.

## In Go

A plugin in Go imports Coddy's sdk, `github.com/akrck02/coddy/sdk`, and gives `Serve` what it
answers: what its actions do, its version control, its review source, or several.

```go
func main() {
	if err := sdk.Serve(sdk.Handlers{RunAction: export}); err != nil {
		log.Fatal(err)
	}
}

func export(params sdk.ActionParams) (sdk.ActionResult, error) {
	// make the PDF of params.File …
	return sdk.ActionResult{Created: pdf}, nil
}
```

The program must write nothing else on its standard output, which is the protocol's: logs go to
standard error, where Coddy's own go. A plugin with settings gives `Handlers.Initialize` a function, which
is handed the values once the program starts and before any other request; it keeps what it
needs. `plugins/github` keeps which pull requests to list. `plugins/github`
answers `sdk.ReviewSource` by running `gh`, as `plugins/azure-devops` does over HTTP.
A plugin in another language follows the same messages, each framed as a language server's are:
a `Content-Length: N` header, an empty line, and N bytes of JSON.

## Installing and trying it

Copy the folder into `plugins/` of Coddy's own folder (`~/.config/coddy/plugins/` on Linux,
`~/Library/Application Support/coddy/plugins/` on a Mac), with the program built inside and its
icon file if it has one, and restart Coddy. `plugins/README.md` has the commands for the official ones and the examples.
Every plugin is on once installed; Settings › Plugins turns it off, and Coddy remembers which
are off (the `plugins-off` preference).

## What is not there yet

Actions only reach the file open, from the menu: they have no keys, and get neither the
selection nor a way to change the text. Pull requests are only read: commenting and approving
wait, and so do pull requests of projects read through a plugin's version control. A plugin's screen gets the pointer but not the keys yet, and
draws no pictures (images, or the lines of paths) and has no tree for screen readers. Each is a `TODO(akrck02)` in the code, waiting for a plugin that needs it.
