package github

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v29/github"
)

// Context contains details on the workflow execution
var Context = parseenv()

// WebhookPayload webhook payload object that triggered the workflow
type WebhookPayload struct {
	*github.PushEvent
	*github.MilestoneEvent
	// TODO: make this work with a simple interface user must be able to access payload.Repository for example
	// *github.CheckRunEvent
	// *github.CheckSuiteEvent
	// *github.CommitCommentEvent
	// *github.CreateEvent
	// *github.DeleteEvent
	// *github.DeployKeyEvent
	// *github.DeploymentEvent
	// *github.DeploymentStatusEvent
	// *github.ForkEvent
	// *github.GollumEvent
	// *github.InstallationEvent
	// *github.InstallationRepositoriesEvent
	// *github.IssueEvent
	// *github.IssueCommentEvent
	// *github.IssuesEvent
	// *github.LabelEvent
	// *github.MarketplacePurchaseEvent
	// *github.MemberEvent
	// *github.MembershipEvent
	// *github.MetaEvent
	// *github.OrgBlockEvent
	// *github.OrganizationEvent
	// *github.PageBuildEvent
	// *github.PingEvent
	// *github.ProjectEvent
	// *github.ProjectColumnEvent
	// *github.ProjectCardEvent
	// *github.PublicEvent
	// *github.PullRequestReviewEvent
	// *github.PullRequestReviewCommentEvent
	// *github.ReleaseEvent
	// *github.RepositoryEvent
	// *github.RepositoryDispatchEvent
	// *github.StarEvent
	// *github.StatusEvent
	// *github.TeamEvent
	// *github.TeamAddEvent
	// *github.UserEvent
	// *github.WatchEvent
	Number       *int                 `json:"number"`
	Label        *github.Label        `json:"label"`
	Repository   *github.Repository   `json:"repository"`
	Issue        *github.Issue        `json:"issue"`
	PullRequest  *github.PullRequest  `json:"pull_request"`
	Sender       *github.Contributor  `json:"sender"`
	Action       string               `json:"action"`
	Installation *github.Installation `json:"installation"`
}

type ActionIssue struct {
	Owner  string
	Repo   string
	Number int
}

type ActionRepo struct {
	Owner string
	Repo  string
}

// ActionContext contains details on the workflow execution
type ActionContext struct {
	Payload   WebhookPayload
	EventName string
	SHA       string
	Ref       string
	Workflow  string
	Action    string
	Actor     string
	Issue     ActionIssue
	Repo      ActionRepo
}

func noGitHubEvent(path string) {
	fmt.Println(fmt.Sprintf("GITHUB_EVENT_PATH %s does not exist", path))
}

func getIndex(a []string, i int) string {
	if len(a) > i {
		return a[i]
	}
	return ""
}

func parseenv() ActionContext {
	r := strings.SplitN(os.Getenv("GITHUB_REPOSITORY"), "/", 2)
	repo := ActionRepo{
		Owner: getIndex(r, 0),
		Repo:  getIndex(r, 1),
	}
	ctx := ActionContext{
		EventName: os.Getenv("GITHUB_EVENT_NAME"),
		SHA:       os.Getenv("GITHUB_SHA"),
		Ref:       os.Getenv("GITHUB_REF"),
		Workflow:  os.Getenv("GITHUB_WORKFLOW"),
		Action:    os.Getenv("GITHUB_ACTION"),
		Actor:     os.Getenv("GITHUB_ACTOR"),
		Repo:      repo,
		Issue: ActionIssue{
			Owner: repo.Owner,
			Repo:  repo.Repo,
		},
	}
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if _, err := os.Stat(eventPath); err == nil && eventPath != "" {
		fd, err := os.Open(eventPath)
		if err != nil {
			noGitHubEvent(eventPath)
		} else {
			json.NewDecoder(fd).Decode(&ctx.Payload)
		}
	} else {
		noGitHubEvent(eventPath)
	}
	if ctx.Payload.Issue != nil && ctx.Payload.Issue.Number != nil {
		ctx.Issue.Number = *ctx.Payload.Issue.Number
	} else if ctx.Payload.PullRequest != nil && ctx.Payload.PullRequest.Number != nil {
		ctx.Issue.Number = *ctx.Payload.PullRequest.Number
	} else if ctx.Payload.Number != nil {
		ctx.Issue.Number = *ctx.Payload.Issue.Number
	}
	if ctx.Payload.Repository != nil {
		ctx.Issue.Owner, ctx.Issue.Repo = ctx.Payload.Repository.GetOwner().GetLogin(), ctx.Payload.Repository.GetName()
		ctx.Repo.Owner, ctx.Repo.Repo = ctx.Issue.Owner, ctx.Issue.Repo
	}
	return ctx
}
