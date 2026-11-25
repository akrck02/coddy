package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/go-renderer/failure"
)

// fakeGh puts on the PATH a gh that prints what GitHub would for one pull request, #12, and one
// comment on it; asked for the person's own pull requests, it gives #14, and fails for anything else, as gh does outside a repository of GitHub.
func fakeGh(test *testing.T) {
	test.Helper()
	folder := test.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
"pr list") case "$*" in
	*"--author @me"*) echo '[{"number":14,"title":"Softer wick","author":{"login":"me"},"baseRefName":"main","headRefName":"softer"}]' ;;
	*) echo '[{"number":12,"title":"Brighter lamp","author":{"login":"keeper"},"baseRefName":"main","headRefName":"brighter"}]' ;;
	esac ;;
"api repos/{owner}/{repo}/pulls/12/comments?per_page=100") echo '[{"path":"src/lamp.go","line":4,"user":{"login":"keeper"},"body":"always on?"},{"path":"old.go","line":null,"user":{"login":"keeper"},"body":"gone"}]' ;;
*) echo "no repository here" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(folder, "gh"), []byte(script), 0o755); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestThePullRequestsAreListedWithAHeadThatCanBeFetched(test *testing.T) {
	fakeGh(test)
	reviews, err := (&github{}).List(test.TempDir())
	want := sdk.Review{ID: "12", Title: "Brighter lamp", Author: "keeper", BaseRef: "main", HeadRef: "pull/12/head"}
	if err != nil || len(reviews) != 1 || reviews[0] != want {
		test.Errorf("the pull requests are %+v (%v)", reviews, err)
	}
}

// The setting "pullRequests" narrows the list to what the person reviews.
func TestThePullRequestsListedAreThoseTheSettingsChose(test *testing.T) {
	fakeGh(test)
	plugin := &github{}
	plugin.configure(map[string]any{"pullRequests": minePulls})
	reviews, err := plugin.List(test.TempDir())
	if err != nil || len(reviews) != 1 || reviews[0].ID != "14" {
		test.Errorf("the person's own pull requests are %+v (%v)", reviews, err)
	}
}

func TestTheCommentsOfAPullRequestKeepTheirFileAndLine(test *testing.T) {
	fakeGh(test)
	comments, err := (&github{}).Comments(sdk.ReviewParams{Folder: test.TempDir(), ID: "12"})
	if err != nil || len(comments) != 2 {
		test.Fatalf("the comments are %+v (%v)", comments, err)
	}
	if comments[0] != (sdk.ReviewComment{File: "src/lamp.go", Line: 4, Author: "keeper", Text: "always on?"}) || comments[1].Line != 0 {
		test.Errorf("the comments read %+v", comments)
	}
}

func TestAFailingGhIsToldWithItsCode(test *testing.T) {
	fakeGh(test)
	if _, err := (&github{}).Comments(sdk.ReviewParams{Folder: test.TempDir(), ID: "13"}); !failure.Is(err, CommandFailed) {
		test.Errorf("gh failing gave %v, want github.command-failed", err)
	}
	if _, err := (&github{}).Comments(sdk.ReviewParams{Folder: test.TempDir(), ID: "12; rm"}); !failure.Is(err, CommandFailed) {
		test.Errorf("an id that is not a number gave %v", err)
	}
}

func TestEachErrorCarriesItsCode(test *testing.T) {
	if !failure.Is(failure.Wrap(CommandFailed, "", nil), CommandFailed) || !failure.Is(failure.Wrap(UnreadableAnswer, "", nil), UnreadableAnswer) {
		test.Error("an error lost its code")
	}
}
