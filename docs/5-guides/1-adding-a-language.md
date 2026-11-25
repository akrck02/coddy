# Adding a language

This guide is for someone who wants Coddy to know one more language: its files' icon, its
colours, its Markdown fences and its language server's name for it. By the end, it is one line.

## The shortest way

Add an entry to `Languages` in `editor/internal/language/language.go`:

```go
{Name: "Zig", Extensions: []string{".zig"}, ServerLanguage: "zig", Fences: []string{"zig"}, Icon: "file-code", Tint: "keyword"},
```

That is all. Every part of the reader asks the table, so the sidebar lists `.zig` files with the
icon, the viewer colours them, a fence named `zig` in a Markdown file is coloured the same, and a
language server that reads `"zig"` (see [adding a language server](2-adding-a-language-server.md))
gets them.

## Each field

| Field | What it says | Example |
|---|---|---|
| `Name` | How people write it | `"Kotlin"` |
| `Extensions` | Its files' endings, lower case, with the dot. A longer one wins | `".gradle.kts"` before `".kts"` |
| `FileNames` | Whole names, lower case, also matched before the first dot | `"dockerfile"` for `Dockerfile.prod` |
| `ServerLanguage` | What the language server protocol calls it; `""` for none | `"kotlin"` |
| `Lexer` | The highlighter's name, only when Chroma does not know the extension | `"groovy"` for `.gradle` |
| `Fences` | Words a Markdown fence may use besides the extension | `"golang"` |
| `Icon`, `Tint` | A Lucide icon and the role in the code's theme it is tinted by | `"file-code"`, `"keyword"` |
| `JVM` | It runs on the Java virtual machine: its imports are read to find uses, and a name may live in a jar | `true` for Kotlin |
| `Classes` | Each file declares a class named as the file, so the sidebar lists it by its class | `true` for Java |

## Checking it

```bash
go test ./internal/language ./internal/highlight ./internal/theme
```

`TestEveryLanguageOfTheTableIsHighlightedIconedAndServedTheSameWay` checks each entry is coloured,
iconed and served as the table says. `TestEachExtensionAndNameBelongsToOneLanguage` fails if your
extension is already another language's. Code elsewhere asks the table (`language.Of`, `IsJVM`,
`ClassName`) instead of writing an extension by hand; a review checks it.

An icon theme chosen in the settings (Catppuccin's, say) draws its own icons over `Icon`; the
table's icon is the built-in one.
