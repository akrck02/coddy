package git

import (
	"context"
	"strings"
)

// remote is the remote pull requests are fetched from.
const remote = "origin"

// Fetch brings head and base from the repository's remote (origin) and gives the commit head
// names, and the base as the remote's branch (origin/main). It only reads the remote; here it adds
// objects, FETCH_HEAD and the remote's branch under .git, and leaves the files and the branches
// checked out as they are.
// TODO(akrck02): another remote than origin, for a fork's pull request.
func Fetch(stop context.Context, repository, head, base string) (headCommit, baseRef string, err error) {
	if _, err := execute(stop, repository, "fetch", "--no-tags", "--quiet", "--end-of-options", remote, head, base); err != nil {
		return "", "", err
	}
	output, err := run(repository, "rev-parse", "--verify", "--quiet", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(string(output)), remote + "/" + base, nil
}
