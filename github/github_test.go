package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/actions-go/toolkit/github"
	"github.com/stretchr/testify/assert"
)

const content = `package toolkit

// placeholder for go docs
`

func TestClient(t *testing.T) {
	repo, _, err := github.GitHub.Repositories.Get(context.Background(), "actions", "toolkit")
	assert.NoError(t, err)
	assert.NotNil(t, repo.Owner)
	assert.NotNil(t, repo.Owner.Login)
	assert.EqualValues(t, "actions", *repo.Owner.Login)
}

func TestDownload(t *testing.T) {
	files := github.DownloadSelectedRepositoryFiles(http.DefaultClient, "actions-go", "toolkit", "09edac1c7d93e0dd7fe5a14dc410fb0b41ea01c4", github.MatchesOneOf("^module.go$"))
	assert.Len(t, files, 1)
	assert.Equal(t, "module.go", files["module.go"].Path)
	assert.Equal(t, []byte(content), files["module.go"].Data)
}

func TestMatchOneOf(t *testing.T) {
	assert.True(t, github.MatchesOneOf("\\.github/settings\\..*")(".github/settings.json"))
	assert.False(t, github.MatchesOneOf("\\.github/settings\\..*")(".github/settings/branches/master/protection.json"))
	assert.True(t, github.MatchesOneOf("\\.github/settings\\..*", ".github/settings/.*")(".github/settings/branches/master/protection.json"))
	assert.False(t, github.MatchesOneOf("\\.github/some-other.*")(".github/settings/branches/master/protection.json"))
}
