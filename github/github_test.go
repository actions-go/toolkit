package github_test

import (
	"context"
	"testing"

	"github.com/actions-go/toolkit/github"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	repo, _, err := github.GitHub.Repositories.Get(context.Background(), "actions", "toolkit")
	assert.NoError(t, err)
	assert.NotNil(t, repo.Owner)
	assert.NotNil(t, repo.Owner.Login)
	assert.EqualValues(t, "actions", *repo.Owner.Login)
}
