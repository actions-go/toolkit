package core

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
}
func TestIssueCommand(t *testing.T) {
	b := bytes.NewBuffer(nil)
	stdout = b
	IssueCommand("hello", map[string]string{
		"some":  "a\n\rvalue,:%",
		"other": "value",
	}, "some\r\n%message")
	assert.Contains(t, b.String(), "some=a%0A%0Dvalue%2C%3A%25")
	assert.Contains(t, b.String(), "other=value")
	assert.Regexp(t, regexp.MustCompile("::some%0D%0A%25message\n$"), b.String())
	assert.Regexp(t, regexp.MustCompile("^::hello "), b.String())
	assert.Len(t, strings.Split(b.String(), ","), 2)

}
