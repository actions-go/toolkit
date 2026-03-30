package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withSummaryFile(t *testing.T) (string, *Summary) {
	t.Helper()
	fd, err := os.CreateTemp("", "summary")
	require.NoError(t, err)
	name := fd.Name()
	fd.Close()
	t.Setenv(GitHubSummaryPathEnvName, name)
	t.Cleanup(func() { os.Remove(name) })
	s := &Summary{}
	return name, s
}

func TestSummaryWrite(t *testing.T) {
	name, s := withSummaryFile(t)
	s.AddRaw("hello world")
	require.NoError(t, s.Write())
	content, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestSummaryWriteOverwrite(t *testing.T) {
	name, s := withSummaryFile(t)
	s.AddRaw("first")
	require.NoError(t, s.Write())
	s.AddRaw("second")
	require.NoError(t, s.Write(SummaryWriteOptions{Overwrite: true}))
	content, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "second", string(content))
}

func TestSummaryWriteAppends(t *testing.T) {
	name, s := withSummaryFile(t)
	s.AddRaw("first")
	require.NoError(t, s.Write())
	s.AddRaw("second")
	require.NoError(t, s.Write())
	content, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "firstsecond", string(content))
}

func TestSummaryClear(t *testing.T) {
	name, s := withSummaryFile(t)
	s.AddRaw("content to clear")
	require.NoError(t, s.Write())
	require.NoError(t, s.Clear())
	content, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "", string(content))
}

func TestSummaryIsEmptyBuffer(t *testing.T) {
	_, s := withSummaryFile(t)
	assert.True(t, s.IsEmptyBuffer())
	s.AddRaw("x")
	assert.False(t, s.IsEmptyBuffer())
	s.EmptyBuffer()
	assert.True(t, s.IsEmptyBuffer())
}

func TestSummaryStringify(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddRaw("hello")
	assert.Equal(t, "hello", s.Stringify())
}

func TestSummaryAddHeading(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddHeading("Title", 1)
	assert.Equal(t, "<h1>Title</h1>"+EOF, s.Stringify())
	s.EmptyBuffer()
	s.AddHeading("Sub", 3)
	assert.Equal(t, "<h3>Sub</h3>"+EOF, s.Stringify())
}

func TestSummaryAddHeadingDefaultLevel(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddHeading("Title")
	assert.Equal(t, "<h1>Title</h1>"+EOF, s.Stringify())
}

func TestSummaryAddSeparator(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddSeparator()
	assert.Equal(t, "<hr>"+EOF, s.Stringify())
}

func TestSummaryAddBreak(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddBreak()
	assert.Equal(t, "<br>"+EOF, s.Stringify())
}

func TestSummaryAddQuote(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddQuote("some quote")
	assert.Equal(t, "<blockquote>some quote</blockquote>"+EOF, s.Stringify())
	s.EmptyBuffer()
	s.AddQuote("cited", "https://example.com")
	assert.Contains(t, s.Stringify(), `cite="https://example.com"`)
}

func TestSummaryAddLink(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddLink("click me", "https://example.com")
	assert.Equal(t, `<a href="https://example.com">click me</a>`+EOF, s.Stringify())
}

func TestSummaryAddList(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddList([]string{"a", "b", "c"})
	assert.Equal(t, "<ul><li>a</li><li>b</li><li>c</li></ul>"+EOF, s.Stringify())
	s.EmptyBuffer()
	s.AddList([]string{"1", "2"}, true)
	assert.Equal(t, "<ol><li>1</li><li>2</li></ol>"+EOF, s.Stringify())
}

func TestSummaryAddTable(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddTable([][]SummaryTableCell{
		{{Data: "H1", Header: true}, {Data: "H2", Header: true}},
		{{Data: "R1C1"}, {Data: "R1C2"}},
	})
	result := s.Stringify()
	assert.Contains(t, result, "<table>")
	assert.Contains(t, result, "<th>H1</th>")
	assert.Contains(t, result, "<td>R1C1</td>")
}

func TestSummaryAddDetails(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddDetails("label", "content")
	assert.Equal(t, "<details><summary>label</summary>content</details>"+EOF, s.Stringify())
}

func TestSummaryAddCodeBlock(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddCodeBlock("fmt.Println()", "go")
	assert.Contains(t, s.Stringify(), `<pre lang="go"><code>fmt.Println()</code></pre>`)
}

func TestSummaryAddImage(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddImage("https://example.com/img.png", "alt text")
	result := s.Stringify()
	assert.Contains(t, result, `src="https://example.com/img.png"`)
	assert.Contains(t, result, `alt="alt text"`)
}

func TestSummaryAddImageWithDimensions(t *testing.T) {
	_, s := withSummaryFile(t)
	s.AddImage("img.png", "alt", SummaryImageOptions{Width: "100", Height: "200"})
	result := s.Stringify()
	assert.Contains(t, result, `width="100"`)
	assert.Contains(t, result, `height="200"`)
}

func TestSummaryNoEnvVar(t *testing.T) {
	t.Setenv(GitHubSummaryPathEnvName, "")
	s := &Summary{}
	s.AddRaw("content")
	err := s.Write()
	assert.Error(t, err)
}
