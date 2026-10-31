package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func parseWithEvent(t *testing.T, event string) ActionContext {
	t.Helper()
	file := filepath.Join(t.TempDir(), "event.json")
	assert.NoError(t, os.WriteFile(file, []byte(event), 0o644))
	t.Setenv("GITHUB_EVENT_PATH", file)
	return ParseActionEnv()
}

func TestParseActionEnvIssueNumber(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	assert.Equal(t, 7, parseWithEvent(t, `{"number": 7}`).Issue.Number)
	assert.Equal(t, 2, parseWithEvent(t, `{"pull_request": {"number": 2}, "number": 3}`).Issue.Number)
	assert.Equal(t, 1, parseWithEvent(t, `{"issue": {"number": 1}, "pull_request": {"number": 2}}`).Issue.Number)
	assert.Equal(t, 0, parseWithEvent(t, `{"issue": {}, "pull_request": {"number": 2}}`).Issue.Number)
}

func TestParseActionEnvRepoPrecedence(t *testing.T) {
	event := `{"repository": {"name": "payload", "owner": {"login": "po"}}}`
	t.Setenv("GITHUB_REPOSITORY", "o/r/extra")
	ctx := parseWithEvent(t, event)
	assert.Equal(t, ActionRepo{Owner: "o", Repo: "r"}, ctx.Repo)
	assert.Equal(t, ActionIssue{Owner: "o", Repo: "r"}, ctx.Issue)

	t.Setenv("GITHUB_REPOSITORY", "")
	ctx = parseWithEvent(t, event)
	assert.Equal(t, ActionRepo{Owner: "po", Repo: "payload"}, ctx.Repo)
}

func TestParseActionEnvRunNumbersLikeParseInt(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", " 12abc")
	t.Setenv("GITHUB_RUN_NUMBER", "-3")
	t.Setenv("GITHUB_RUN_ATTEMPT", "abc")
	ctx := ParseActionEnv()
	assert.EqualValues(t, 12, ctx.RunID)
	assert.EqualValues(t, -3, ctx.RunNumber)
	assert.EqualValues(t, 0, ctx.RunAttempt)
}

func TestUserAgentCountsUTF16Units(t *testing.T) {
	bom, nextLine, eAcute, grinning := string(rune(0xFEFF)), string(rune(0x85)), string(rune(0xE9)), string(rune(0x1F600))
	t.Setenv("ACTIONS_ORCHESTRATION_ID", bom+"x"+grinning+"y"+eAcute+nextLine)
	assert.Equal(t, "actions_orchestration_id/x__y__", UserAgentWithOrchestrationID(""))
}
