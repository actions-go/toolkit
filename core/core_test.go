package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
