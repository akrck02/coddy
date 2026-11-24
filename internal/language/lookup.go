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
)

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
