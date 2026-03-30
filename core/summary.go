package core

import (
	"fmt"
	"os"
	"strings"
)

// SummaryTableCell represents a cell in a summary table.
type SummaryTableCell struct {
	// Data is the cell content.
	Data string
	// Header renders cell as header (<th>) when true.
	Header bool
	// Colspan is the number of columns the cell extends (optional, default "1").
	Colspan string
	// Rowspan is the number of rows the cell extends (optional, default "1").
	Rowspan string
}

// SummaryImageOptions optional attributes for summary images.
type SummaryImageOptions struct {
	// Width in pixels (integer, no unit).
	Width string
	// Height in pixels (integer, no unit).
	Height string
}

// SummaryWriteOptions controls write behavior.
type SummaryWriteOptions struct {
	// Overwrite replaces all existing content when true (default: false, appends).
	Overwrite bool
}

// Summary is a builder for GitHub Actions job summaries.
// Use the package-level JobSummary variable rather than constructing directly.
type Summary struct {
	buffer   strings.Builder
	filePath string
}

// JobSummary is the package-level summary instance, equivalent to core.summary in the JS toolkit.
var JobSummary = &Summary{}

func (s *Summary) getFilePath() (string, error) {
	if s.filePath != "" {
		return s.filePath, nil
	}
	p, ok := os.LookupEnv(GitHubSummaryPathEnvName)
	if !ok || p == "" {
		return "", fmt.Errorf("unable to find environment variable for $%s. Check if your runtime environment supports job summaries", GitHubSummaryPathEnvName)
	}
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("unable to access summary file: %q. Check if the file has correct read/write permissions", p)
	}
	s.filePath = p
	return s.filePath, nil
}

func (s *Summary) wrap(tag, content string, attrs map[string]string) string {
	var attrStr strings.Builder
	for k, v := range attrs {
		fmt.Fprintf(&attrStr, ` %s="%s"`, k, v)
	}
	if content == "" {
		return fmt.Sprintf("<%s%s>", tag, attrStr.String())
	}
	return fmt.Sprintf("<%s%s>%s</%s>", tag, attrStr.String(), content, tag)
}

// Write flushes the buffer to the summary file and clears the buffer.
// Appends by default; set options.Overwrite to replace existing content.
func (s *Summary) Write(options ...SummaryWriteOptions) error {
	overwrite := len(options) > 0 && options[0].Overwrite
	filePath, err := s.getFilePath()
	if err != nil {
		return err
	}
	flag := os.O_APPEND | os.O_WRONLY | os.O_CREATE
	if overwrite {
		flag = os.O_TRUNC | os.O_WRONLY | os.O_CREATE
	}
	fd, err := os.OpenFile(filePath, flag, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fmt.Fprint(fd, s.buffer.String())
	if err != nil {
		return err
	}
	s.EmptyBuffer()
	return nil
}

// Clear empties the buffer and wipes the summary file.
func (s *Summary) Clear() error {
	s.EmptyBuffer()
	return s.Write(SummaryWriteOptions{Overwrite: true})
}

// Stringify returns the current buffer content as a string.
func (s *Summary) Stringify() string {
	return s.buffer.String()
}

// IsEmptyBuffer reports whether the buffer is empty.
func (s *Summary) IsEmptyBuffer() bool {
	return s.buffer.Len() == 0
}

// EmptyBuffer resets the buffer without writing to the file.
func (s *Summary) EmptyBuffer() *Summary {
	s.buffer.Reset()
	return s
}

// AddRaw adds raw text to the buffer.
func (s *Summary) AddRaw(text string, addEOL ...bool) *Summary {
	s.buffer.WriteString(text)
	if len(addEOL) > 0 && addEOL[0] {
		s.AddEOL()
	}
	return s
}

// AddEOL appends an OS-specific end-of-line marker to the buffer.
func (s *Summary) AddEOL() *Summary {
	return s.AddRaw(EOF)
}

// AddCodeBlock adds an HTML code block to the buffer.
// lang is an optional language for syntax highlighting.
func (s *Summary) AddCodeBlock(code string, lang ...string) *Summary {
	attrs := map[string]string{}
	if len(lang) > 0 && lang[0] != "" {
		attrs["lang"] = lang[0]
	}
	element := s.wrap("pre", s.wrap("code", code, nil), attrs)
	return s.AddRaw(element, true)
}

// AddList adds an HTML list to the buffer.
// ordered controls whether an <ol> or <ul> is rendered (default: unordered).
func (s *Summary) AddList(items []string, ordered ...bool) *Summary {
	tag := "ul"
	if len(ordered) > 0 && ordered[0] {
		tag = "ol"
	}
	var listItems strings.Builder
	for _, item := range items {
		listItems.WriteString(s.wrap("li", item, nil))
	}
	element := s.wrap(tag, listItems.String(), nil)
	return s.AddRaw(element, true)
}

// AddTable adds an HTML table to the buffer.
// Each row is a slice of SummaryTableCell or string values.
func (s *Summary) AddTable(rows [][]SummaryTableCell) *Summary {
	var tableBody strings.Builder
	for _, row := range rows {
		var cells strings.Builder
		for _, cell := range row {
			tag := "td"
			if cell.Header {
				tag = "th"
			}
			attrs := map[string]string{}
			if cell.Colspan != "" {
				attrs["colspan"] = cell.Colspan
			}
			if cell.Rowspan != "" {
				attrs["rowspan"] = cell.Rowspan
			}
			cells.WriteString(s.wrap(tag, cell.Data, attrs))
		}
		tableBody.WriteString(s.wrap("tr", cells.String(), nil))
	}
	element := s.wrap("table", tableBody.String(), nil)
	return s.AddRaw(element, true)
}

// AddDetails adds a collapsible HTML details element.
func (s *Summary) AddDetails(label, content string) *Summary {
	element := s.wrap("details", s.wrap("summary", label, nil)+content, nil)
	return s.AddRaw(element, true)
}

// AddImage adds an HTML image tag.
func (s *Summary) AddImage(src, alt string, options ...SummaryImageOptions) *Summary {
	attrs := map[string]string{"src": src, "alt": alt}
	if len(options) > 0 {
		if options[0].Width != "" {
			attrs["width"] = options[0].Width
		}
		if options[0].Height != "" {
			attrs["height"] = options[0].Height
		}
	}
	element := s.wrap("img", "", attrs)
	return s.AddRaw(element, true)
}

// AddHeading adds an HTML heading element (h1–h6). Level defaults to 1.
func (s *Summary) AddHeading(text string, level ...int) *Summary {
	lvl := 1
	if len(level) > 0 && level[0] >= 1 && level[0] <= 6 {
		lvl = level[0]
	}
	tag := fmt.Sprintf("h%d", lvl)
	element := s.wrap(tag, text, nil)
	return s.AddRaw(element, true)
}

// AddSeparator adds an HTML thematic break (<hr>).
func (s *Summary) AddSeparator() *Summary {
	return s.AddRaw(s.wrap("hr", "", nil), true)
}

// AddBreak adds an HTML line break (<br>).
func (s *Summary) AddBreak() *Summary {
	return s.AddRaw(s.wrap("br", "", nil), true)
}

// AddQuote adds an HTML blockquote. cite is an optional citation URL.
func (s *Summary) AddQuote(text string, cite ...string) *Summary {
	attrs := map[string]string{}
	if len(cite) > 0 && cite[0] != "" {
		attrs["cite"] = cite[0]
	}
	element := s.wrap("blockquote", text, attrs)
	return s.AddRaw(element, true)
}

// AddLink adds an HTML anchor tag.
func (s *Summary) AddLink(text, href string) *Summary {
	element := s.wrap("a", text, map[string]string{"href": href})
	return s.AddRaw(element, true)
}
