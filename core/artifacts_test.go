package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclareArtifact(t *testing.T) {
	t.Run("appends trimmed declarations to the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "artifacts")
		require.NoError(t, os.WriteFile(path, nil, 0600))
		t.Setenv(GitHubArtifactsEnvName, path)

		require.NoError(t, DeclareArtifact("  dist/app.tar.gz \n"))
		require.NoError(t, DeclareArtifact("oci://ghcr.io/octocat/app:1.0.0@sha256:abc"))

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "dist/app.tar.gz\noci://ghcr.io/octocat/app:1.0.0@sha256:abc\n", string(content))
	})
	t.Run("is a no-op when GITHUB_ARTIFACTS is unset", func(t *testing.T) {
		t.Setenv(GitHubArtifactsEnvName, "")
		assert.NoError(t, DeclareArtifact("dist/app.tar.gz"))
	})
	t.Run("rejects invalid values", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "artifacts")
		require.NoError(t, os.WriteFile(path, nil, 0600))
		t.Setenv(GitHubArtifactsEnvName, path)

		assert.ErrorContains(t, DeclareArtifact("  "), "must not be empty")
		assert.ErrorContains(t, DeclareArtifact("a\nb"), "line breaks")
		assert.ErrorContains(t, DeclareArtifact("#foo"), "comment")
		assert.NoError(t, DeclareArtifact("file://#foo"))
	})
}
