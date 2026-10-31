package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgentWithOrchestrationID(t *testing.T) {
	t.Run("unset returns the base", func(t *testing.T) {
		t.Setenv("ACTIONS_ORCHESTRATION_ID", "  ")
		assert.Equal(t, "base", UserAgentWithOrchestrationID("base"))
		assert.Equal(t, "", UserAgentWithOrchestrationID(""))
	})
	t.Run("appends the sanitized tag", func(t *testing.T) {
		t.Setenv("ACTIONS_ORCHESTRATION_ID", "a b/c.d-e_f")
		assert.Equal(t, "base actions_orchestration_id/a_b_c.d-e_f", UserAgentWithOrchestrationID("base"))
		assert.Equal(t, "actions_orchestration_id/a_b_c.d-e_f", UserAgentWithOrchestrationID(""))
	})
	t.Run("does not duplicate the tag", func(t *testing.T) {
		t.Setenv("ACTIONS_ORCHESTRATION_ID", "id1")
		ua := UserAgentWithOrchestrationID("base")
		assert.Equal(t, ua, UserAgentWithOrchestrationID(ua))
	})
	t.Run("NewClient applies it", func(t *testing.T) {
		t.Setenv("ACTIONS_ORCHESTRATION_ID", "id1")
		assert.Contains(t, NewClient().UserAgent, "actions_orchestration_id/id1")
	})
}

func TestParseActionEnvRef(t *testing.T) {
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_REF_TYPE", "branch")
	ctx := ParseActionEnv()
	assert.Equal(t, "main", ctx.RefName)
	assert.Equal(t, "branch", ctx.RefType)
}
