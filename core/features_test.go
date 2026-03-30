package core

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	Notice("a notice message")
	assert.Equal(t, "::notice::a notice message\n", b.String())
}

func TestNoticeWithAnnotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	Notice("annotated", AnnotationProperties{
		Title: "My Title",
		File:  "src/main.go",
	})
	result := b.String()
	assert.Contains(t, result, "::notice ")
	assert.Contains(t, result, "title=My Title")
	assert.Contains(t, result, "file=src/main.go")
	assert.Contains(t, result, "::annotated")
}

func TestNoticef(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	Noticef("notice %s %d", "msg", 42)
	assert.Equal(t, "::notice::notice msg 42\n", b.String())
}

func TestErrorWithAnnotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	Error("bad thing", AnnotationProperties{
		File:      "main.go",
		StartLine: 10,
		EndLine:   12,
	})
	result := b.String()
	assert.Contains(t, result, "::error ")
	assert.Contains(t, result, "file=main.go")
	assert.Contains(t, result, "line=10")
	assert.Contains(t, result, "endLine=12")
}

func TestWarningWithAnnotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	Warning("careful", AnnotationProperties{
		File:        "pkg/foo.go",
		StartColumn: 5,
		EndColumn:   15,
	})
	result := b.String()
	assert.Contains(t, result, "::warning ")
	assert.Contains(t, result, "file=pkg/foo.go")
	assert.Contains(t, result, "col=5")
	assert.Contains(t, result, "endColumn=15")
}

func TestSetCommandEcho(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	SetCommandEcho(true)
	assert.Equal(t, "::echo::on\n", b.String())

	b.Reset()
	SetCommandEcho(false)
	assert.Equal(t, "::echo::off\n", b.String())
}

func TestGetMultilineInput(t *testing.T) {
	// Use a discard buffer so Debug calls don't hit nil stdout
	discard := bytes.NewBuffer(nil)

	t.Run("not set returns empty slice", func(t *testing.T) {
		origLookup := lookupEnv
		origStdout := stdout
		t.Cleanup(func() { lookupEnv = origLookup; stdout = origStdout })
		stdout = discard
		lookupEnv = func(name string) (string, bool) { return "", false }
		result := GetMultilineInput("my-input")
		assert.Equal(t, []string{}, result)
	})

	t.Run("single line", func(t *testing.T) {
		origLookup := lookupEnv
		t.Cleanup(func() { lookupEnv = origLookup })
		lookupEnv = func(name string) (string, bool) { return "hello", true }
		result := GetMultilineInput("my-input")
		assert.Equal(t, []string{"hello"}, result)
	})

	t.Run("multiple lines", func(t *testing.T) {
		origLookup := lookupEnv
		t.Cleanup(func() { lookupEnv = origLookup })
		lookupEnv = func(name string) (string, bool) { return "  line1  \n  line2  \nline3\n", true }
		result := GetMultilineInput("my-input")
		assert.Equal(t, []string{"line1", "line2", "line3"}, result)
	})

	t.Run("filters empty lines", func(t *testing.T) {
		origLookup := lookupEnv
		t.Cleanup(func() { lookupEnv = origLookup })
		lookupEnv = func(name string) (string, bool) { return "a\n\nb\n", true }
		result := GetMultilineInput("my-input")
		assert.Equal(t, []string{"a", "b"}, result)
	})
}

func TestPathUtils(t *testing.T) {
	assert.Equal(t, "foo/bar/baz", ToPosixPath(`foo\bar\baz`))
	assert.Equal(t, `foo\bar\baz`, ToWin32Path("foo/bar/baz"))
}

func TestAnnotationPropertiesEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test only runs on unix with \\n line separator")
	}
	b := bytes.NewBuffer(nil)
	stdout = b
	origStdout := stdout
	t.Cleanup(func() { stdout = origStdout })

	// Without annotation properties falls back to simple Issue call
	Error("plain error")
	assert.Equal(t, "::error::plain error\n", b.String())
}
