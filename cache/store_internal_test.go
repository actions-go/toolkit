package cache

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoRelKeepsATargetInsideTheToolFolder(t *testing.T) {
	assert.Equal(t, "escaped.txt", noRel("../escaped.txt"))
	assert.Equal(t, filepath.Join("a", "b"), noRel("./../a/b"))
	assert.Equal(t, "escaped.txt", noRel(filepath.FromSlash("../escaped.txt")))
}
