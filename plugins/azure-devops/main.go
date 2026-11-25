// The azure-devops plugin lets Coddy review the pull requests of a repository of Azure DevOps: it
// answers the review source messages of Coddy's sdk by reading Azure DevOps' REST API, with a
// personal access token. It only reads: it never comments, votes or changes anything there.
package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/go-renderer/failure"
)

// requestSeconds is how long a request to Azure DevOps may take before it is given up: Coddy
// waits for the answer.
const requestSeconds = 30

// apiVersion is the version of Azure DevOps' REST API the plugin reads.
const apiVersion = "7.1"

// tokenVariable is the environment variable of a token, when the settings have none: the one
// Azure's own command reads.
const tokenVariable = "AZURE_DEVOPS_EXT_PAT"

// branchPrefix starts the name Azure gives of a branch: refs/heads/main.
const branchPrefix = "refs/heads/"

// systemComment is the commentType of what Azure writes itself on a thread: a vote, a push.
const systemComment = "system"

// TODO(akrck02): more than the first page of pull requests (Azure gives 101 at most without
// $top and $skip); comments on a whole file, which have no line.

func main() {
	plugin := newAzure()
	if err := sdk.Serve(sdk.Handlers{Initialize: plugin.configure, ReviewSource: plugin}); err != nil {
		log.Fatal(err)
	}
}

// azure answers Coddy's questions about pull requests with Azure DevOps' REST API. token and
// organizationURL are what the person set in its settings.
type azure struct {
	token, organizationURL string
	client                 *http.Client
}

// newAzure is the plugin before its settings are given, its requests given up after
// requestSeconds.
func newAzure() *azure {
	return &azure{client: &http.Client{Timeout: requestSeconds * time.Second}}
}

// configure keeps the settings Coddy gave as the plugin started.
func (plugin *azure) configure(settings map[string]any) {
	token, _ := settings["token"].(string) // anything else is no token: the environment's is used
	address, _ := settings["organizationUrl"].(string)
	plugin.token, plugin.organizationURL = strings.TrimSpace(token), strings.TrimSpace(address)
}

// pullRequest is a pull request as Azure DevOps gives it.
type pullRequest struct {
	PullRequestID int    `json:"pullRequestId"`
	Title         string `json:"title"`
	CreatedBy     person `json:"createdBy"`
	SourceRefName string `json:"sourceRefName"`
	TargetRefName string `json:"targetRefName"`
}

// person is someone as Azure DevOps names them.
type person struct {
	DisplayName string `json:"displayName"`
}

// List gives the active pull requests of the folder's repository. Each head is its source
// branch, which Coddy fetches from the remote origin.
func (plugin *azure) List(folder string) ([]sdk.Review, error) {
	found, err := plugin.repositoryOf(folder)
	if err != nil {
		return nil, err
	}
	var answer struct {
		Value []pullRequest `json:"value"`
	}
	if err := plugin.get(found.address("pullrequests", "searchCriteria.status=active"), &answer); err != nil {
		return nil, err
	}
	reviews := make([]sdk.Review, len(answer.Value))
	for index, each := range answer.Value {
		reviews[index] = sdk.Review{ID: strconv.Itoa(each.PullRequestID), Title: each.Title,
			Author:  each.CreatedBy.DisplayName,
			BaseRef: strings.TrimPrefix(each.TargetRefName, branchPrefix),
			HeadRef: strings.TrimPrefix(each.SourceRefName, branchPrefix)}
	}
	return reviews, nil
}

// thread is a thread of comments of a pull request, as Azure DevOps gives it. Its context is
// nil for a thread on the whole pull request.
type thread struct {
	IsDeleted     bool `json:"isDeleted"`
	ThreadContext *struct {
		FilePath       string    `json:"filePath"`
		RightFileStart *position `json:"rightFileStart"`
		LeftFileStart  *position `json:"leftFileStart"`
	} `json:"threadContext"`
	Comments []struct {
		Author      person `json:"author"`
		Content     string `json:"content"`
		CommentType string `json:"commentType"`
		IsDeleted   bool   `json:"isDeleted"`
	} `json:"comments"`
}

// position is a place in a file, its line counted from 1.
type position struct {
	Line int `json:"line"`
}

// line is the line a thread is on: on the file as the pull request has it, or else on the file
// as it was, for a line removed; 0 when it has neither.
func (each thread) line() int {
	switch {
	case each.ThreadContext.RightFileStart != nil:
		return each.ThreadContext.RightFileStart.Line
	case each.ThreadContext.LeftFileStart != nil:
		return each.ThreadContext.LeftFileStart.Line
	}
	return 0
}

// Comments gives the comments people wrote on the files of a pull request: not those on the
// whole pull request, not those deleted, not those Azure writes itself.
func (plugin *azure) Comments(asked sdk.ReviewParams) ([]sdk.ReviewComment, error) {
	if _, err := strconv.Atoi(asked.ID); err != nil {
		return nil, failure.Wrap(RequestFailed, "not a pull request's number: "+asked.ID, err)
	}
	found, err := plugin.repositoryOf(asked.Folder)
	if err != nil {
		return nil, err
	}
	var answer struct {
		Value []thread `json:"value"`
	}
	if err := plugin.get(found.address("pullRequests/"+asked.ID+"/threads", ""), &answer); err != nil {
		return nil, err
	}
	var comments []sdk.ReviewComment
	for _, each := range answer.Value {
		if each.IsDeleted || each.ThreadContext == nil || each.ThreadContext.FilePath == "" {
			continue
		}
		file, line := strings.TrimPrefix(each.ThreadContext.FilePath, "/"), each.line()
		for _, written := range each.Comments {
			if written.IsDeleted || written.CommentType == systemComment {
				continue
			}
			comments = append(comments, sdk.ReviewComment{File: file, Line: line,
				Author: written.Author.DisplayName, Text: written.Content})
		}
	}
	return comments, nil
}

// repositoryOf is where the repository of a folder is on Azure DevOps, read from its remote.
func (plugin *azure) repositoryOf(folder string) (repository, error) {
	remote, err := remoteOf(folder)
	if err != nil {
		return repository{}, err
	}
	return locate(remote, plugin.organizationURL)
}

// address is the URL of something of the repository's in the REST API, with a query of its own
// ("" for none) before the API's version.
func (found repository) address(path, query string) string {
	address := found.base + "/" + url.PathEscape(found.project) + "/_apis/git/repositories/" + url.PathEscape(found.name) + "/" + path + "?"
	if query != "" {
		address += query + "&"
	}
	return address + "api-version=" + apiVersion
}

// tokenOf is the token to read Azure DevOps with: the settings', or else the environment's.
func (plugin *azure) tokenOf() (string, error) {
	if plugin.token != "" {
		return plugin.token, nil
	}
	if token := strings.TrimSpace(os.Getenv(tokenVariable)); token != "" {
		return token, nil
	}
	return "", NoToken
}

// authorization is the header that gives Azure DevOps a personal access token: Basic, with an
// empty user name.
func authorization(token string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+token))
}

// get asks the REST API for an address and reads its JSON into answer.
func (plugin *azure) get(address string, answer any) error {
	token, err := plugin.tokenOf()
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return failure.Wrap(RequestFailed, "", err)
	}
	request.Header.Set("Authorization", authorization(token))
	request.Header.Set("Accept", "application/json")
	response, err := plugin.client.Do(request)
	if err != nil {
		return failure.Wrap(RequestFailed, "", err)
	}
	defer func() {
		_ = response.Body.Close() // what was read is read: closing only frees the connection
	}()
	switch {
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNonAuthoritativeInfo:
		return Unauthorized
	case response.StatusCode != http.StatusOK:
		return failure.Wrap(RequestFailed, "status "+response.Status, nil)
	}
	if err := json.NewDecoder(response.Body).Decode(answer); err != nil {
		return failure.Wrap(RequestFailed, "an answer that is not the JSON asked for", err)
	}
	return nil
}
