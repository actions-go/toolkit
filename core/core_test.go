package core

import (
	"bytes"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestSummary(t *testing.T) {
	t.Run("integrated with Github Actions (it should appear on the run)", func(t *testing.T) {
		AddStepSummary("and a new line")
		DeleteStepSummary()
		AddStepSummary("this content should be overridden")
		ReplaceStepSummary("this is the new content")
		AddStepSummary("and a new line")
	})
	t.Run("with content check", func(t *testing.T) {
		fd, err := os.CreateTemp("", "summary")
		require.NoError(t, err)
		name := fd.Name()
		t.Cleanup(func() {
			fd.Close()
			os.Remove(name)
		})
		t.Setenv(GitHubSummaryPathEnvName, name)

		AddStepSummary("and a new line")
		DeleteStepSummary()
		AddStepSummary("this content should be overridden")
		ReplaceStepSummary("this is the new content")
		AddStepSummary("and a new line")
		content, err := os.ReadFile(name)
		require.NoError(t, err)
		assert.Equal(t, "this is the new content\nand a new line\n", string(content))
	})
}

func TestStopCommand(t *testing.T) {
	defer func() {
		stdout = os.Stdout
	}()
	WithoutCommands("temporary", func() {
		Error("this should not make the test to fail")
	})
	out := bytes.Buffer{}
	stdout = &out

	t.Run("stop command is written on stdout (test written for unix only)", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("This test only runs on unix with \\n line separator")
		}
		called := false
		WithoutCommands("temporary", func() {
			called = true
			Error("en-error")
		})
		assert.True(t, called)
		assert.Equal(t, "::stop-commands::temporary\n::error::en-error\n::temporary::\n", out.String())
	})
}

func TestFormatOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	assert.Equal(t, "my-name<<_GitHubActionsGoFileCommandDelimeter_\nmy-value\n_GitHubActionsGoFileCommandDelimeter_\n", formatOutput("my-name", "my-value"))
}

func TestOutputTasks(t *testing.T) {
	if _, ok := os.LookupEnv("ACTIONS_OUTPUT_SET"); ok {
		// state is only available in pre and post actions:
		// https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#sending-values-to-the-pre-and-post-actions
		// assert.Equal(t, "my-state-value", GetState("my-state"))
		assert.Equal(t, "my-output-value", os.Getenv("my_output"))
		assert.Equal(t, "my-env-value", os.Getenv("my_env"))
	}
	SaveState("my-state", "my-state-value")
	ExportVariable("my_env", "my-env-value")
	SetOutput("my-output", "my-output-value")
}

func TestGetInput(t *testing.T) {
	t.Run("when environment variable is not net", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return "", false
		}
		v, ok := GetInput("some-input with-space")
		assert.False(t, ok)
		assert.Equal(t, "", v)
	})
	t.Run("when environment variable is not net", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return " some value that needs to be Trimmed \n", true
		}
		v, ok := GetInput("some-input with-space")
		assert.True(t, ok)
		assert.Equal(t, "some value that needs to be Trimmed", v)
	})
}

func TestInputDefault(t *testing.T) {
	t.Run("when environment variable is not net", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return "", false
		}
		v := GetInputOrDefault("some-input with-space", " default value not trimmed ")
		assert.Equal(t, " default value not trimmed ", v)
	})
	t.Run("when environment variable is not net", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return " some value that needs to be Trimmed \n", true
		}
		v := GetInputOrDefault("some-input with-space", "some default not used")
		assert.Equal(t, "some value that needs to be Trimmed", v)
	})
}

func TestBoolInput(t *testing.T) {
	t.Run("when environment variable is not net", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return "", false
		}
		assert.False(t, GetBoolInput("some-input with-space"))
	})
	t.Run("when environment variable is 'false'", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return "false", true
		}
		assert.False(t, GetBoolInput("some-input with-space"))
	})
	t.Run("when environment variable is 'True'", func(t *testing.T) {
		lookupEnv = func(name string) (string, bool) {
			t.Run("environment variable lookup should be uppercase without space", func(t *testing.T) {
				assert.Equal(t, "INPUT_SOME-INPUT_WITH-SPACE", name)
			})
			return "True", true
		}
		assert.True(t, GetBoolInput("some-input with-space"))
	})
}
