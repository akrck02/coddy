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
| `-code-font file.ttf` | Read the code in this TrueType face (default SF Mono, from macOS; Go Mono where there is none) |
| `-all` | Show every project, not only those with changes |
| `-read-only` | Start read-only: files are only read, not edited |
| `-light` | Start in the light theme (dark by default) |
| `-width`, `-height`, `-capture shot.png` | go-ui's window flags; `-capture` draws one frame to a picture |

```bash
go vet ./... && go test ./...     # the interface tests drive the window with no display
```
