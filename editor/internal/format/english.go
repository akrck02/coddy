// Package format turns values into text for people to read: a count of things in the singular or
// the plural, a sentence's first capital. Its packages format code (highlight) and Markdown.
package format

import (
	"strconv"
	"strings"
)

// Count writes a count of things, the word in the plural unless there is one: "1 project",
// "22 projects", "2 old indexes".
func Count(count int, thing string) string {
	if count == 1 {
		return "1 " + thing
	}
	return strconv.Itoa(count) + " " + Plural(thing)
}

// Plural is a word in the plural, by the regular rule: "es" after s, x, z, ch and sh, "s" after
// the rest. Irregular words ("child") are not asked for.
func Plural(thing string) string {
	for _, ending := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(thing, ending) {
			return thing + "es"
		}
	}
	return thing + "s"
}

// Capitalised is a word with its first letter in capitals: "starting" gives "Starting".
func Capitalised(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}
