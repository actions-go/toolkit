package github

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/actions-go/toolkit/core"
	"github.com/google/go-github/v42/github"
)

// Context contains details on the workflow execution
var Context = ParseActionEnv()

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
	Payload           WebhookPayload
	EventName         string
	SHA               string
	Ref               string
	RefName           string
	RefType           string
	Workflow          string
	Action            string
	Actor             string
	Job               string
	RunAttempt        int64
	RunNumber         int64
	RunID             int64
	ApiUrl            string
	ServerUrl         string
	GraphqlUrl        string
	Issue             ActionIssue
	Repo              ActionRepo
	OutputFilePath    string
	StateFilePath     string
	ExportEnvFilePath string
}

func noGitHubEvent(path string) {
	// @actions/github terminates the message with os.EOL
	fmt.Printf("GITHUB_EVENT_PATH %s does not exist%s", path, core.EOF)
}

func getIndex(a []string, i int) string {
	if len(a) > i {
		return a[i]
	}
	return ""
}

var leadingInteger = regexp.MustCompile(`^[ \t\n\v\f\r\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]*[+-]?[0-9]+`)

// parseInt64Env reads the variable like JavaScript's parseInt(value, 10): the
// leading integer is used and anything after it is ignored. It returns 0 when
// there is none, where JavaScript gives NaN.
func parseInt64Env(name string) int64 {
	digits := leadingInteger.FindString(os.Getenv(name))
	v, err := strconv.ParseInt(strings.TrimLeftFunc(digits, isJSSpace), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func getenvOrDefault(name, defaultVal string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return defaultVal
}

// ParseActionEnv parses the environment and extracts the ActionContext on demand. For example in tests
func ParseActionEnv() ActionContext {
	repo := repoFromEnv()
	ctx := ActionContext{
		EventName:         os.Getenv("GITHUB_EVENT_NAME"),
		SHA:               os.Getenv("GITHUB_SHA"),
		Ref:               os.Getenv("GITHUB_REF"),
		RefName:           os.Getenv("GITHUB_REF_NAME"),
		RefType:           os.Getenv("GITHUB_REF_TYPE"),
		Workflow:          os.Getenv("GITHUB_WORKFLOW"),
		Action:            os.Getenv("GITHUB_ACTION"),
		Actor:             os.Getenv("GITHUB_ACTOR"),
		Job:               os.Getenv("GITHUB_JOB"),
		RunAttempt:        parseInt64Env("GITHUB_RUN_ATTEMPT"),
		RunNumber:         parseInt64Env("GITHUB_RUN_NUMBER"),
		RunID:             parseInt64Env("GITHUB_RUN_ID"),
		ApiUrl:            getenvOrDefault("GITHUB_API_URL", "https://api.github.com"),
		ServerUrl:         getenvOrDefault("GITHUB_SERVER_URL", "https://github.com"),
		GraphqlUrl:        getenvOrDefault("GITHUB_GRAPHQL_URL", "https://api.github.com/graphql"),
		OutputFilePath:    os.Getenv(core.GitHubOutputFilePathEnvName),
		StateFilePath:     os.Getenv(core.GitHubStateFilePathEnvName),
		ExportEnvFilePath: os.Getenv(core.GitHubExportEnvFilePathEnvName),
		Repo:              repo,
	}
	// like @actions/github, nothing is said when GITHUB_EVENT_PATH is unset or empty
	if eventPath := os.Getenv("GITHUB_EVENT_PATH"); eventPath != "" {
		if fd, err := os.Open(eventPath); err != nil {
			noGitHubEvent(eventPath)
		} else {
			defer fd.Close()
			json.NewDecoder(fd).Decode(&ctx.Payload)
		}
	}
	// GITHUB_REPOSITORY takes precedence over the repository of the payload
	if repo == (ActionRepo{}) && ctx.Payload.Repository != nil {
		ctx.Repo.Owner, ctx.Repo.Repo = ctx.Payload.Repository.GetOwner().GetLogin(), ctx.Payload.Repository.GetName()
	}
	ctx.Issue = ActionIssue{Owner: ctx.Repo.Owner, Repo: ctx.Repo.Repo, Number: issueNumber(ctx.Payload)}
	return ctx
}

// repoFromEnv splits GITHUB_REPOSITORY into owner and repository. Segments
// after the second are dropped.
func repoFromEnv() ActionRepo {
	r := strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")
	return ActionRepo{Owner: getIndex(r, 0), Repo: getIndex(r, 1)}
}

// issueNumber is the number of the first of the issue, the pull request or the
// payload itself that is present, even when that one has no number.
func issueNumber(p WebhookPayload) int {
	var n *int
	switch {
	case p.Issue != nil:
		n = p.Issue.Number
	case p.PullRequest != nil:
		n = p.PullRequest.Number
	default:
		n = p.Number
	}
	if n == nil {
		return 0
	}
	return *n
}
