# Get started

You need Go 1.25 or later and the Xcode command line tools (`xcode-select --install`): the window
is opened through cgo and drawn with Metal.

```bash
go build -o bin/code-review ./cmd/code-review
./bin/code-review /path/to/workspace                 # a folder of repositories
./bin/code-review ../api ../web                      # or the repositories themselves
./bin/code-review -base origin/release/2.4 /path/to/workspace
```

| Flag | |
|---|---|
| `-base ref` | Compare every project against this branch, tag or commit |
| `-committed` | Compare the last commit only, leaving out changes not committed yet |
| `-open project/path` | Open a file at the start |
| `-review` | Start in the review mode |
| `-find text` | Start with the finder searching the files' content for a text |
| `-code-font file.ttf` | Read the code in this TrueType face (default the system's: SF Mono on a Mac, the desktop's monospaced font on Linux; Go Mono where there is none) |
| `-all` | Show every project, even if only the changes were shown last time |
| `-read-only` | Start read-only: files are only read, not edited |
| `-light` | Start in One Light (the theme is chosen in the settings otherwise, and remembered) |
| `-width`, `-height`, `-capture shot.png` | go-ui's window flags; `-capture` draws one frame to a picture |

```bash
go vet ./... && go test ./...     # the interface tests drive the window with no display
```

## Bring your icons 🎨

The file icons can come from any icon theme made for Zed, such as
[Catppuccin's](https://github.com/catppuccin/zed-icons). If Zed is installed, its icon themes are
listed already. Otherwise, copy the extension's folder (the one holding `icon_themes/`) into the
reader's `icon-themes/` folder, beside `themes/`: on a Mac
`~/Library/Application Support/code-review/icon-themes/`, on Linux
`~/.config/code-review/icon-themes/`. Start the reader, open the settings (`⌘,`) and choose one
under *File icons*; it is remembered.
