package github

import (
	"context"
	"net/http"
	"os"

	"github.com/google/go-github/v29/github"
	"golang.org/x/oauth2"
)

func NewClient() *github.Client {
	token := os.Getenv("GITHUB_TOKEN")
	httpClient := http.DefaultClient
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	return github.NewClient(httpClient)
}

var GitHub = NewClient()
