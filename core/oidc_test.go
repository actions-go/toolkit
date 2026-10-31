package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// the expected values are those of JavaScript's encodeURIComponent
func TestEncodeURIComponent(t *testing.T) {
	assert.Equal(t, "a%20b%26c%3Dd%2F%C3%A9!~*'()%2B", encodeURIComponent("a b&c=d/é!~*'()+"))
	assert.Equal(t, "https%3A%2F%2Fexample.com%2Faud", encodeURIComponent("https://example.com/aud"))
	assert.Equal(t, "", encodeURIComponent(""))
}

// the expected values are those of the WHATWG URL parser (what Node sends)
func TestEscapeQuery(t *testing.T) {
	assert.Equal(t, "http://h/p?x=a%20b%22c%3Cd%3E%27e&y=%C3%A9", escapeQuery("http://h/p?x=a b\"c<d>'e&y=é"))
	assert.Equal(t, "http://h/p a", escapeQuery("http://h/p a"), "only the query is touched")
	assert.Equal(t, "http://h/p?a%20b#frag ment", escapeQuery("http://h/p?a b#frag ment"), "the fragment is left alone")
}
