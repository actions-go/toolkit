package core

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DeclareArtifact declares an artifact produced by the current step.
//
// Each call declares a single artifact, which is either of the following:
//
//   - A path to a local file, such as `dist/app.tar.gz`. The runner resolves
//     relative paths against GITHUB_WORKSPACE, not the current working directory.
//   - An OCI subject reference that includes a digest, such as
//     `ghcr.io/octocat/app:1.0.0@sha256:<hex>`.
//
// To force how the runner interprets the value, prefix it with `file://` or `oci://`.
//
// Artifact declarations require runner version 2.336.0 or later. On older runners
// GITHUB_ARTIFACTS is not set, so a debug message is logged and nil is returned.
func DeclareArtifact(artifact string) error {
	value := strings.TrimSpace(artifact)

	// The runner reads one declaration per line and skips blank and comment
	// lines, so reject values that would be split or silently dropped.
	if value == "" {
		return fmt.Errorf("artifact must not be empty")
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("artifact must not contain line breaks: %s", strconv.Quote(value))
	}
	if strings.HasPrefix(value, "#") {
		return fmt.Errorf("artifact %s would be ignored as a comment. To declare a file whose name starts with '#', prefix the path with 'file://'", strconv.Quote(value))
	}

	if os.Getenv(GitHubArtifactsEnvName) == "" {
		Debugf("Skipping artifact declaration %s: $%s is not set. Artifact declarations require runner version 2.336.0 or later.", strconv.Quote(value), GitHubArtifactsEnvName)
		return nil
	}

	return issueFileCommand(GitHubArtifactsEnvName, value)
}
