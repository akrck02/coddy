// The github plugin lets Coddy review the pull requests of a GitHub repository: it answers the
// review source messages of Coddy's sdk by running gh, GitHub's command, which keeps the person's
// login. It only reads: it never comments, approves or changes anything on GitHub.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/go-renderer/failure"
)

// pullFields are the fields gh pr list gives of each pull request.
const pullFields = "number,title,author,baseRefName,headRefName"

// TODO(akrck02): more than the first 100 pull requests and comments (gh's --paginate); comments
// on a whole file, and on lines the pull request no longer has, which GitHub gives without a line.

// The choices of the setting "pullRequests", as plugin.json lists them.
const (
	everyOpenPull   = "Every open one"
	minePulls       = "Mine"
	reviewRequested = "Asking for my review"
)

// pullFilters are the flags of gh pr list that keep only the pull requests chosen.
var pullFilters = map[string][]string{
	minePulls:       {"--author", "@me"},
	reviewRequested: {"--search", "review-requested:@me"},
}

func main() {
	plugin := &github{}
	if err := sdk.Serve(sdk.Handlers{Initialize: plugin.configure, ReviewSource: plugin}); err != nil {
		log.Fatal(err)
	}
}

// github answers Coddy's questions about pull requests with gh's commands. pulls is which pull
// requests it lists, as the person chose in its settings.
type github struct {
	pulls string
}

// configure keeps the settings Coddy gave as the plugin started.
func (plugin *github) configure(settings map[string]any) {
	plugin.pulls, _ = settings["pullRequests"].(string) // anything else lists every open one
}

// pull is a pull request as gh pr list gives it.
type pull struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
}

// List gives the open pull requests. Each head is GitHub's ref of the pull request, pull/N/head,
// which can be fetched from the repository even when it comes from a fork.
// Only those the setting "pullRequests" chose are listed: every open one, those by the person, or
// those asking for their review.
func (plugin *github) List(folder string) ([]sdk.Review, error) {
	var pulls []pull
	arguments := append([]string{"pr", "list", "--state", "open", "--limit", "100", "--json", pullFields}, pullFilters[plugin.pulls]...)
	if err := ghJSON(folder, &pulls, arguments...); err != nil {
		return nil, err
	}
	reviews := make([]sdk.Review, len(pulls))
	for index, each := range pulls {
		id := strconv.Itoa(each.Number)
		reviews[index] = sdk.Review{ID: id, Title: each.Title, Author: each.Author.Login,
			BaseRef: each.BaseRefName, HeadRef: "pull/" + id + "/head"}
	}
	return reviews, nil
}

// comment is a comment on a line of a pull request, as GitHub's API gives it.
type comment struct {
	Path string `json:"path"`
	Line int    `json:"line"` // 0 (null) for a line the pull request no longer has
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	Body string `json:"body"`
}

// Comments gives the comments on the lines of a pull request. gh fills {owner}/{repo} with the
// repository of the folder.
func (*github) Comments(asked sdk.ReviewParams) ([]sdk.ReviewComment, error) {
	if _, err := strconv.Atoi(asked.ID); err != nil {
		return nil, failure.Wrap(CommandFailed, "not a pull request's number: "+asked.ID, err)
	}
	var answered []comment
	if err := ghJSON(asked.Folder, &answered, "api", "repos/{owner}/{repo}/pulls/"+asked.ID+"/comments?per_page=100"); err != nil {
		return nil, err
	}
	comments := make([]sdk.ReviewComment, len(answered))
	for index, each := range answered {
		comments[index] = sdk.ReviewComment{File: each.Path, Line: each.Line, Author: each.User.Login, Text: each.Body}
	}
	return comments, nil
}

// ghJSON runs gh in a folder and reads what it prints as JSON into answer.
func ghJSON(folder string, answer any, arguments ...string) error {
	command := exec.Command("gh", arguments...)
	command.Dir = folder
	var output, errorOutput bytes.Buffer
	command.Stdout, command.Stderr = &output, &errorOutput
	if err := command.Run(); err != nil {
		return failure.Wrap(CommandFailed, "gh "+arguments[0]+": "+strings.TrimSpace(errorOutput.String()), err)
	}
	if err := json.Unmarshal(output.Bytes(), answer); err != nil {
		return failure.Wrap(UnreadableAnswer, "gh "+arguments[0], err)
	}
	return nil
}
