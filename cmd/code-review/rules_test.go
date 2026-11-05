package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot is the folder of go.mod, seen from this package's folder.
const moduleRoot = "../.."

// goSources gives each Go file of the module that is not a test, with its text.
func goSources(test *testing.T) map[string]string {
	test.Helper()
	sources := map[string]string{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources[path] = string(content)
		return nil
	})
	if err != nil {
		test.Fatal(err)
	}
	return sources
}

// Every error carries a code from failure, so no error is made any other way.
func TestNoErrorIsMadeWithoutACode(test *testing.T) {
	for path, source := range goSources(test) {
		for _, maker := range []string{"errors.New(", "fmt.Errorf("} {
			if strings.Contains(source, maker) {
				test.Errorf("%s makes an error with %s; declare a code in its errors.go", path, maker)
			}
		}
	}
}
