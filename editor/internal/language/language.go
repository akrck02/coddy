// Package language is what Coddy knows of each language: how its files are named, what a language
// server calls it, how its icon looks, what a Markdown fence calls it and which highlighter reads
// it. It is one table, so adding a language is adding one entry, and every part of the reader
// (the sidebar, the highlighting, the servers, the Markdown) treats it the same way.
package language

// Language is a language, or a kind of text file, as Coddy knows it.
type Language struct {
	// Name is how people write it: "Kotlin".
	Name string
	// Extensions are its files' extensions, in lower case with their dot; the first is the one a
	// Markdown fence's code is coloured as. A longer one wins: ".gradle.kts" before ".kts".
	Extensions []string
	// FileNames are its files' whole names in lower case, also matched before a file's first dot
	// ("Dockerfile.prod").
	FileNames []string
	// ServerLanguage is what the language server protocol calls it ("kotlin"); "" for none.
	ServerLanguage string
	// Lexer is the highlighter's name for it, given only where the highlighter does not know the
	// extension by itself.
	Lexer string
	// Fences are the words a Markdown fence names it by, besides its extension: "kotlin" for ".kt".
	Fences []string
	// Icon is Lucide's icon its files are listed with ("" for the plain file icon), and Tint the
	// colour of the code it is drawn in, by its role in the code's theme ("keyword").
	Icon string
	Tint string
	// JVM tells whether it runs on the Java virtual machine: its files import names by package,
	// and a name they use may be declared in a jar.
	JVM bool
	// Classes tells whether each of its files declares a class named as the file ("Lamp.kt"
	// declares Lamp), so the file can be listed as its class.
	Classes bool
}

// The Names of the languages the reader treats in a way of their own.
const (
	// Markdown can be shown formatted.
	Markdown = "Markdown"
	// Go, Java and Kotlin have the language servers the reader knows.
	Go     = "Go"
	Java   = "Java"
	Kotlin = "Kotlin"
)

// Languages are every language Coddy knows, in the order a server's language is named by.
var Languages = []Language{
	{Name: Java, Extensions: []string{".java"}, ServerLanguage: "java", Icon: "coffee", Tint: "number", JVM: true, Classes: true},
	{Name: Kotlin, Extensions: []string{".kt"}, ServerLanguage: "kotlin", Fences: []string{"kotlin"}, Icon: "file-code", Tint: "keyword", JVM: true, Classes: true},
	{Name: "Kotlin script", Extensions: []string{".kts"}, ServerLanguage: "kotlin", Lexer: "kotlin", Icon: "file-cog", Tint: "keyword", JVM: true},
	{Name: "Gradle", Extensions: []string{".gradle"}, Lexer: "groovy", Fences: []string{"gradle"}, Icon: "file-cog", Tint: "string"},
	{Name: "Gradle in Kotlin", Extensions: []string{".gradle.kts"}, ServerLanguage: "kotlin", Lexer: "kotlin", Icon: "file-cog", Tint: "string", JVM: true},
	{Name: Go, Extensions: []string{".go"}, ServerLanguage: "go", Fences: []string{"golang"}, Icon: "file-code", Tint: "type"},
	{Name: "TypeScript", Extensions: []string{".ts", ".tsx"}, ServerLanguage: "typescript", Fences: []string{"typescript"}, Icon: "file-code", Tint: "function"},
	{Name: "JavaScript", Extensions: []string{".js", ".mjs", ".jsx"}, ServerLanguage: "javascript", Fences: []string{"javascript"}, Icon: "file-code", Tint: "constant"},
	{Name: "Python", Extensions: []string{".py"}, ServerLanguage: "python", Fences: []string{"python"}, Icon: "file-code", Tint: "function"},
	{Name: "Rust", Extensions: []string{".rs"}, ServerLanguage: "rust", Fences: []string{"rust"}, Icon: "file-code", Tint: "number"},
	{Name: "Ruby", Extensions: []string{".rb"}, Fences: []string{"ruby"}},
	{Name: "Vue", Extensions: []string{".vue"}, Lexer: "vue", Icon: "file-code", Tint: "string"},
	{Name: "C", Extensions: []string{".c", ".h"}, Icon: "file-code", Tint: "function"},
	{Name: "C++", Extensions: []string{".cpp"}, Icon: "file-code", Tint: "function"},
	{Name: "C#", Extensions: []string{".cs"}, Fences: []string{"csharp", "c#"}, Icon: "file-code", Tint: "keyword"},
	{Name: "Swift", Extensions: []string{".swift"}, Icon: "file-code", Tint: "number"},
	{Name: "Dart", Extensions: []string{".dart"}, Icon: "file-code", Tint: "type"},
	{Name: "Groovy", Extensions: []string{".groovy"}, Fences: []string{"groovy"}, Icon: "file-code", Tint: "type"},
	{Name: "Scala", Extensions: []string{".scala"}, Icon: "file-code", Tint: "property"},
	{Name: "HTML", Extensions: []string{".html"}, Icon: "code-xml", Tint: "property"},
	{Name: "XML", Extensions: []string{".xml"}, Icon: "code-xml", Tint: "property"},
	{Name: "CSS", Extensions: []string{".css"}, Icon: "hash", Tint: "function"},
	{Name: "SCSS", Extensions: []string{".scss"}, Icon: "hash", Tint: "keyword"},
	{Name: "Less", Extensions: []string{".less"}, Icon: "hash", Tint: "function"},
	{Name: "JSON", Extensions: []string{".json"}, Icon: "braces", Tint: "constant"},
	{Name: "YAML", Extensions: []string{".yaml", ".yml"}, Fences: []string{"yml"}, Icon: "file-cog", Tint: "property"},
	{Name: "TOML", Extensions: []string{".toml"}, Icon: "file-cog", Tint: "property"},
	{Name: "Properties", Extensions: []string{".properties"}, Lexer: "properties", Icon: "file-cog", Tint: "comment"},
	{Name: "INI", Extensions: []string{".ini", ".cfg"}, Icon: "file-cog", Tint: "comment"},
	{Name: "Configuration", Extensions: []string{".conf"}, Icon: "file-cog", Tint: "comment"},
	{Name: "Environment", Extensions: []string{".env"}, Lexer: "bash", Icon: "file-key", Tint: "constant"},
	{Name: "SQL", Extensions: []string{".sql"}, Icon: "database", Tint: "function"},
	{Name: Markdown, Extensions: []string{".md", ".markdown", ".mdown", ".mkd"}, Lexer: "markdown", Fences: []string{"markdown"}, Icon: "book-open", Tint: "attribute"},
	{Name: "Plain text", Extensions: []string{".txt"}, Icon: "file-text", Tint: "comment"},
	{Name: "Shell", Extensions: []string{".sh", ".bash", ".zsh"}, Fences: []string{"bash", "shell", "zsh"}, Icon: "file-terminal", Tint: "string"},
	{Name: "Batch", Extensions: []string{".bat"}, Icon: "file-terminal", Tint: "string"},
	{Name: "PowerShell", Extensions: []string{".ps1"}, Icon: "file-terminal", Tint: "string"},
	{Name: "Dockerfile", FileNames: []string{"dockerfile"}, Lexer: "docker", Icon: "container", Tint: "function"},
	{Name: "Makefile", FileNames: []string{"makefile"}, Icon: "file-terminal", Tint: "string"},
}
