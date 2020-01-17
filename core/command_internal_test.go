package core

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIssue(t *testing.T) {
	b := bytes.NewBuffer(nil)
	stdout = b
	Issue("hello", "some\r\nmessage")
	assert.Equal(t, "::hello::some%0D%0Amessage\n", b.String())
	b = bytes.NewBuffer(nil)
	stdout = b
	Issue("hello")
	assert.Equal(t, "::hello::\n", b.String())
	b = bytes.NewBuffer(nil)
	stdout = b
	IssueCommand("hello", map[string]string{
		"some": "a\n\rvalue];",
	}, "some\r\nmessage")
	assert.Equal(t, "::hellosome=a%0A%0Dvalue%5D%3B::some%0D%0Amessage\n", b.String())
}
