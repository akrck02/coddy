package language

import (
	"path/filepath"
	"strings"
)

// The table's indexes, made once from the literal above: a lookup allocates nothing but the
// lower-case copy of a name that has capitals.
var (
	byExtension = indexBy(func(language Language) []string { return language.Extensions })
	byFileName  = indexBy(func(language Language) []string { return language.FileNames })
	byFence     = indexBy(func(language Language) []string { return language.Fences })
	// classExtensions are the extensions of the languages whose files are each a class.
	classExtensions = extensionsOfClasses()
)

// extensionsOfClasses lists the extensions of every language whose files are each a class.
func extensionsOfClasses() []string {
	var extensions []string
	for _, language := range Languages {
		if language.Classes {
			extensions = append(extensions, language.Extensions...)
		}
	}
	return extensions
}

// indexBy maps each key of the table's languages, as the function gives them, to its language.
func indexBy(keys func(Language) []string) map[string]*Language {
	index := map[string]*Language{}
	for position := range Languages {
		for _, key := range keys(Languages[position]) {
			index[key] = &Languages[position]
		}
	}
	return index
}

// Of is the language of a file, by its name or its path: its whole name first, then its
// extensions, the longest first, then what comes before its first dot.
func Of(path string) (Language, bool) {
	name := strings.ToLower(filepath.Base(path))
	if language, found := byFileName[name]; found {
		return *language, true
	}
	for index := 0; index < len(name); index++ {
		if name[index] != '.' {
			continue
		}
		if language, found := byExtension[name[index:]]; found {
			return *language, true
		}
	}
	stem, _, _ := strings.Cut(name, ".")
	if language, found := byFileName[stem]; found {
		return *language, true
	}
	return Language{}, false
}

// ServerLanguageOf is what the language server protocol calls a file's language, "" when no
// server is known for it.
func ServerLanguageOf(path string) string {
	language, _ := Of(path) // a file of no language has no server
	return language.ServerLanguage
}

// OfFence is the language a Markdown fence names, by one of its fence words or its extension
// without the dot ("kotlin", "kt").
func OfFence(word string) (Language, bool) {
	word = strings.ToLower(word)
	if language, found := byFence[word]; found {
		return *language, true
	}
	if language, found := byExtension["."+word]; found && word != "" {
		return *language, true
	}
	return Language{}, false
}

// NameOfServerLanguage is how people write the language a server's protocol calls by a name
// ("Kotlin" for "kotlin"); "" when the table does not know it.
func NameOfServerLanguage(serverLanguage string) string {
	for _, language := range Languages {
		if language.ServerLanguage == serverLanguage {
			return language.Name
		}
	}
	return ""
}

// Named is the language of the table with a name ("Kotlin").
func Named(name string) (Language, bool) {
	for _, language := range Languages {
		if language.Name == name {
			return language, true
		}
	}
	return Language{}, false
}

// ServerLanguages are what the language server protocol calls the languages of some names,
// for a server that reads them: ServerLanguages(Kotlin) is ["kotlin"].
func ServerLanguages(names ...string) []string {
	languages := make([]string, 0, len(names))
	for _, name := range names {
		if language, found := Named(name); found && language.ServerLanguage != "" {
			languages = append(languages, language.ServerLanguage)
		}
	}
	return languages
}

// IsJVM tells whether a file is of a language of the Java virtual machine, whose imports name
// packages.
func IsJVM(path string) bool {
	language, _ := Of(path) // a file of no language is not one
	return language.JVM
}

// ClassExtensions are the extensions of the files that are each a class (".java", ".kt").
func ClassExtensions() []string { return classExtensions }

// IsClassFile tells whether a file is one class, named as the file, by its extension as written.
func IsClassFile(path string) bool {
	_, found := classExtensionOf(path)
	return found
}

// ClassName is the name of the class a class file declares: its name without the extension.
func ClassName(fileName string) string {
	extension, _ := classExtensionOf(fileName) // a file that is no class keeps its whole name
	return strings.TrimSuffix(fileName, extension)
}

// classExtensionOf is the extension that makes a file a class, if one does.
func classExtensionOf(path string) (string, bool) {
	for _, extension := range classExtensions {
		if strings.HasSuffix(path, extension) {
			return extension, true
		}
	}
	return "", false
}
