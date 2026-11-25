package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/go-renderer/failure"
)

// The forms of the remotes of one made-up repository, "keeper", of the project "Lamp Room" of the
// organization "lighthouse".
func TestEveryFormOfAnAzureRemoteNamesItsRepository(test *testing.T) {
	want := repository{base: "https://dev.azure.com/lighthouse", project: "Lamp Room", name: "keeper"}
	remotes := []string{
		"https://dev.azure.com/lighthouse/Lamp%20Room/_git/keeper",
		"https://lighthouse@dev.azure.com/lighthouse/Lamp%20Room/_git/keeper",
		"https://lighthouse.visualstudio.com/Lamp%20Room/_git/keeper",
		"https://lighthouse.visualstudio.com/DefaultCollection/Lamp%20Room/_git/keeper",
		"git@ssh.dev.azure.com:v3/lighthouse/Lamp%20Room/keeper",
		"lighthouse@vs-ssh.visualstudio.com:v3/lighthouse/Lamp%20Room/keeper",
	}
	for _, remote := range remotes {
		if found, err := locate(remote, ""); err != nil || found != want {
			test.Errorf("%s names %+v (%v)", remote, found, err)
		}
	}
}

func TestARemoteThatIsNotAzureIsRefused(test *testing.T) {
	for _, remote := range []string{"https://github.com/lighthouse/keeper.git", "git@github.com:lighthouse/keeper.git",
		"https://dev.azure.com/lighthouse/keeper", "/srv/harbour/keeper", ""} {
		if _, err := locate(remote, ""); !failure.Is(err, NotAzure) {
			test.Errorf("%q gave %v, want azure-devops.not-azure", remote, err)
		}
	}
}

// A server on a company's own machine has paths of its own before the project: its collection.
func TestWithAnOrganizationURLTheProjectAndRepositoryComeFromTheRemotesPath(test *testing.T) {
	found, err := locate("https://harbour.example/tfs/Lighthouse/Lamp%20Room/_git/keeper", "https://harbour.example/tfs/Lighthouse/")
	want := repository{base: "https://harbour.example/tfs/Lighthouse", project: "Lamp Room", name: "keeper"}
	if err != nil || found != want {
		test.Errorf("the repository is %+v (%v)", found, err)
	}
}

// harbour is a made-up Azure DevOps: a server that answers the REST API for the project
// "Lamp Room" and its repository "keeper", with a folder whose remote origin points to it. It
// keeps the Authorization header of the last request.
type harbour struct {
	folder        string
	authorization string
	status        int
}

// pullRequests are two active pull requests, as Azure gives them.
const pullRequests = `{"count": 2, "value": [
	{"pullRequestId": 7, "title": "Brighter lamp", "createdBy": {"displayName": "keeper"},
	 "sourceRefName": "refs/heads/feature/brighter", "targetRefName": "refs/heads/main"},
	{"pullRequestId": 9, "title": "Softer wick", "createdBy": {"displayName": "harbour"},
	 "sourceRefName": "refs/heads/softer", "targetRefName": "refs/heads/release/2"}]}`

// threads are the threads of pull request 7: one kept on each side of the file, and those to skip.
const threads = `{"count": 5, "value": [
	{"id": 1, "threadContext": {"filePath": "/src/lamp.go", "rightFileStart": {"line": 4, "offset": 1}},
	 "comments": [{"author": {"displayName": "keeper"}, "content": "always on?", "commentType": "text"},
	              {"author": {"displayName": "harbour"}, "content": "gone", "commentType": "text", "isDeleted": true}]},
	{"id": 2, "threadContext": {"filePath": "/src/wick.go", "leftFileStart": {"line": 10, "offset": 1}},
	 "comments": [{"author": {"displayName": "harbour"}, "content": "why remove it?", "commentType": "text"}]},
	{"id": 3, "threadContext": null,
	 "comments": [{"author": {"displayName": "keeper"}, "content": "on the whole pull request", "commentType": "text"}]},
	{"id": 4, "isDeleted": true, "threadContext": {"filePath": "/src/lamp.go", "rightFileStart": {"line": 1}},
	 "comments": [{"author": {"displayName": "keeper"}, "content": "deleted thread", "commentType": "text"}]},
	{"id": 5, "threadContext": {"filePath": "/src/lamp.go", "rightFileStart": {"line": 2}},
	 "comments": [{"author": {"displayName": "Azure"}, "content": "keeper voted 10", "commentType": "system"}]}]}`

// newHarbour starts the made-up server, and a plugin configured to read it with a token.
func newHarbour(test *testing.T) (*harbour, *azure) {
	test.Helper()
	server := &harbour{folder: test.TempDir(), status: http.StatusOK}
	routes := map[string]string{
		"/Lamp%20Room/_apis/git/repositories/keeper/pullrequests?searchCriteria.status=active&api-version=7.1": pullRequests,
		"/Lamp%20Room/_apis/git/repositories/keeper/pullRequests/7/threads?api-version=7.1":                    threads,
	}
	running := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		server.authorization = request.Header.Get("Authorization")
		answer, found := routes[request.URL.EscapedPath()+"?"+request.URL.RawQuery]
		switch {
		case server.status != http.StatusOK:
			writer.WriteHeader(server.status)
		case !found:
			http.NotFound(writer, request)
		default:
			_, _ = writer.Write([]byte(answer)) // a test's answer; the client tells if it is cut
		}
	}))
	test.Cleanup(running.Close)
	for _, arguments := range [][]string{{"init", "--quiet"}, {"remote", "add", "origin", running.URL + "/Lamp%20Room/_git/keeper"}} {
		if output, err := exec.Command("git", append([]string{"-C", server.folder}, arguments...)...).CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v %s", arguments, err, output)
		}
	}
	plugin := newAzure()
	plugin.configure(map[string]any{"token": "lamp-oil", "organizationUrl": running.URL})
	return server, plugin
}

func TestThePullRequestsAreListedWithTheirBranches(test *testing.T) {
	server, plugin := newHarbour(test)
	reviews, err := plugin.List(server.folder)
	want := []sdk.Review{
		{ID: "7", Title: "Brighter lamp", Author: "keeper", BaseRef: "main", HeadRef: "feature/brighter"},
		{ID: "9", Title: "Softer wick", Author: "harbour", BaseRef: "release/2", HeadRef: "softer"},
	}
	if err != nil || len(reviews) != 2 || reviews[0] != want[0] || reviews[1] != want[1] {
		test.Errorf("the pull requests are %+v (%v)", reviews, err)
	}
}

// Coddy shows comments beside lines of files: one on the whole pull request has no place there,
// and what was deleted or written by Azure itself is not what people said.
func TestOnlyTheCommentsPeopleWroteOnFilesAreGiven(test *testing.T) {
	server, plugin := newHarbour(test)
	comments, err := plugin.Comments(sdk.ReviewParams{Folder: server.folder, ID: "7"})
	want := []sdk.ReviewComment{
		{File: "src/lamp.go", Line: 4, Author: "keeper", Text: "always on?"},
		{File: "src/wick.go", Line: 10, Author: "harbour", Text: "why remove it?"},
	}
	if err != nil || len(comments) != 2 || comments[0] != want[0] || comments[1] != want[1] {
		test.Errorf("the comments are %+v (%v)", comments, err)
	}
	if _, err := plugin.Comments(sdk.ReviewParams{Folder: server.folder, ID: "7/../../x"}); !failure.Is(err, RequestFailed) {
		test.Errorf("an id that is not a number gave %v", err)
	}
}

// Azure takes a personal access token as the password of Basic authentication, with no user.
func TestTheTokenIsSentAsBasicAuthenticationWithoutAUser(test *testing.T) {
	server, plugin := newHarbour(test)
	if _, err := plugin.List(server.folder); err != nil {
		test.Fatal(err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(":lamp-oil"))
	if server.authorization != want || want != "Basic OmxhbXAtb2ls" {
		test.Errorf("the header is %q, want %q", server.authorization, want)
	}
}

// The settings are where the person chose a token for Coddy; the environment's may be for other
// tools.
func TestTheSettingsTokenComesBeforeTheEnvironments(test *testing.T) {
	server, plugin := newHarbour(test)
	test.Setenv(tokenVariable, "harbour-token")
	if _, err := plugin.List(server.folder); err != nil || server.authorization != authorization("lamp-oil") {
		test.Errorf("with both, the header is %q (%v)", server.authorization, err)
	}
	plugin.token = ""
	if _, err := plugin.List(server.folder); err != nil || server.authorization != authorization("harbour-token") {
		test.Errorf("with only the environment's, the header is %q (%v)", server.authorization, err)
	}
}

func TestWithoutATokenNothingIsAsked(test *testing.T) {
	server, plugin := newHarbour(test)
	test.Setenv(tokenVariable, "")
	plugin.token = ""
	if _, err := plugin.List(server.folder); !failure.Is(err, NoToken) || server.authorization != "" {
		test.Errorf("without a token, the list gave %v, and the server was asked with %q", err, server.authorization)
	}
}

// Azure answers a bad token with 401, or with 203 and its sign-in page, which is not the JSON.
func TestATokenRefusedIsUnauthorized(test *testing.T) {
	server, plugin := newHarbour(test)
	for _, status := range []int{http.StatusUnauthorized, http.StatusNonAuthoritativeInfo} {
		server.status = status
		if _, err := plugin.List(server.folder); !failure.Is(err, Unauthorized) {
			test.Errorf("a %d gave %v, want azure-devops.unauthorized", status, err)
		}
	}
	server.status = http.StatusInternalServerError
	if _, err := plugin.List(server.folder); !failure.Is(err, RequestFailed) {
		test.Errorf("a 500 gave %v, want azure-devops.request-failed", err)
	}
}

func TestAFolderWithoutAnAzureRemoteIsNotAzure(test *testing.T) {
	plugin := newAzure()
	plugin.configure(map[string]any{"token": "lamp-oil"})
	if _, err := plugin.List(test.TempDir()); !failure.Is(err, NotAzure) {
		test.Errorf("a folder without a remote gave %v, want azure-devops.not-azure", err)
	}
}
