package main

import (
	"bytes"
	"net/url"
	"os/exec"
	"strings"

	"github.com/akrck02/go-renderer/failure"
)

// The hosts of Azure DevOps' remotes.
const (
	azureHost             = "dev.azure.com"
	azureSSHHost          = "ssh.dev.azure.com"
	visualStudioSSHHost   = "vs-ssh.visualstudio.com"
	visualStudioSuffix    = ".visualstudio.com"
	sshVersion            = "v3"
	gitSegment            = "_git"
	defaultCollection     = "DefaultCollection"
	azureOrganizationsURL = "https://" + azureHost + "/"
)

// repository is where a repository of Azure DevOps is: the base URL of its organization (or of
// a server's collection), and its project's and its own names.
type repository struct {
	base, project, name string
}

// remoteOf is the URL of a folder's remote origin, as git gives it.
func remoteOf(folder string) (string, error) {
	command := exec.Command("git", "-C", folder, "remote", "get-url", "origin")
	var output, errorOutput bytes.Buffer
	command.Stdout, command.Stderr = &output, &errorOutput
	if err := command.Run(); err != nil {
		return "", failure.Wrap(NotAzure, "no remote origin: "+strings.TrimSpace(errorOutput.String()), err)
	}
	return strings.TrimSpace(output.String()), nil
}

// locate is where the repository of a remote is. With an organization URL, for Azure DevOps
// Server on a company's own machine, that URL is the base and the remote's path ends in
// <project>/_git/<repository>; without one, the remote is one of Azure DevOps' own forms:
//
//	https://[user@]dev.azure.com/<organization>/<project>/_git/<repository>
//	https://<organization>.visualstudio.com/[DefaultCollection/]<project>/_git/<repository>
//	git@ssh.dev.azure.com:v3/<organization>/<project>/<repository>
//	<organization>@vs-ssh.visualstudio.com:v3/<organization>/<project>/<repository>
//
// The remote is never put in an error: its URL may carry a token.
func locate(remote, organizationURL string) (repository, error) {
	host, segments, err := split(remote)
	if err != nil {
		return repository{}, err
	}
	if organizationURL != "" {
		return onServer(organizationURL, segments)
	}
	switch {
	case host == azureHost && len(segments) == 4 && segments[2] == gitSegment:
		return repository{base: azureOrganizationsURL + url.PathEscape(segments[0]), project: segments[1], name: segments[3]}, nil
	case (host == azureSSHHost || host == visualStudioSSHHost) && len(segments) == 4 && segments[0] == sshVersion:
		return repository{base: azureOrganizationsURL + url.PathEscape(segments[1]), project: segments[2], name: segments[3]}, nil
	case strings.HasSuffix(host, visualStudioSuffix):
		if len(segments) > 0 && strings.EqualFold(segments[0], defaultCollection) {
			segments = segments[1:]
		}
		organization := strings.TrimSuffix(host, visualStudioSuffix)
		if len(segments) == 3 && segments[1] == gitSegment && organization != "" {
			return repository{base: azureOrganizationsURL + url.PathEscape(organization), project: segments[0], name: segments[2]}, nil
		}
	}
	return repository{}, NotAzure
}

// onServer is a repository of a server whose organization URL the person set: the last three
// segments of the remote's path are <project>/_git/<repository>, whatever collection is before.
func onServer(organizationURL string, segments []string) (repository, error) {
	count := len(segments)
	if count < 3 || segments[count-2] != gitSegment {
		return repository{}, NotAzure
	}
	return repository{base: strings.TrimSuffix(organizationURL, "/"), project: segments[count-3], name: segments[count-1]}, nil
}

// split is a remote's host and the segments of its path, decoded ("Lamp%20Room" is "Lamp Room"):
// a URL (https://, ssh://) or git's short form for SSH, user@host:path.
func split(remote string) (host string, segments []string, err error) {
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return "", nil, failure.Wrap(NotAzure, "the remote origin is not a URL", nil) // not err: it quotes the URL
		}
		return parsed.Hostname(), strings.Split(strings.Trim(parsed.Path, "/"), "/"), nil
	}
	address, path, found := strings.Cut(remote, ":")
	if !found {
		return "", nil, NotAzure
	}
	host = address[strings.LastIndex(address, "@")+1:]
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return "", nil, NotAzure
		}
		segments = append(segments, decoded)
	}
	return host, segments, nil
}
