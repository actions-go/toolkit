package github

import (
	"context"
	"net/http"
	"os"

	"github.com/actions-go/toolkit/core"
	"github.com/google/go-github/v29/github"
	"golang.org/x/oauth2"
)

func token() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	for _, input := range []string{"github-token", "token"} {
		if t, ok := core.GetInput(input); ok {
			return t
		}
	}
	return ""
}

func NewClient() *github.Client {
	token := token()
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
