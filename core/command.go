package core

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	cmdString = "::"
)

var (
	stdout       io.Writer = os.Stdout
	stdoutSetter sync.Mutex
	dataEscapes  = map[string]string{
		"\r": "%0D",
		"\n": "%0A",
	}
	escapes = map[string]string{
		":": "%3A",
		",": "%2C",
	}
)

func SetStdout(w io.Writer) {
	stdoutSetter.Lock()
	stdout = w
	stdoutSetter.Unlock()
}

// Issue displays a plain typed message following github actions interface
func Issue(kind string, message ...string) {
	IssueCommand(kind, nil, strings.Join(message, ""))
}

// IssueCommand displays a typed message with properties following github actions interface.
// see https://github.com/actions/toolkit/blob/e69833ed16500afaa7d137a9cf6da76fb8fb54da/packages/core/src/command.ts#L19
func IssueCommand(kind string, properties map[string]string, message string) {
	c := &command{kind, properties, message}
	stdoutSetter.Lock()
	fmt.Fprintln(stdout, c.String())
	stdoutSetter.Unlock()
}

// issueFileCommand implements stores the command in a file
// see https://github.com/actions/toolkit/pull/571/files#diff-9ce6eb99f5fb5529e795254801e03ae56d67d3d5fcbec635f91e9a8a61ad8b64R10
func issueFileCommandWithPerm(command string, message string, flag int, perm os.FileMode) error {
	path, ok := os.LookupEnv(command)
	if ok {
		fd, err := os.OpenFile(path, flag, perm)
		if err != nil {
			return err
		}
		defer fd.Close()
		fmt.Fprintln(fd, message)
		return nil
	}
	return fmt.Errorf("unable to find command file %s", command)
}

// issueFileCommand implements stores the command in a file
// see https://github.com/actions/toolkit/pull/571/files#diff-9ce6eb99f5fb5529e795254801e03ae56d67d3d5fcbec635f91e9a8a61ad8b64R10
func issueFileCommand(command string, message string) error {
	err := issueFileCommandWithPerm(command, message, os.O_APPEND|os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return nil
}

type command struct {
	command    string
	properties map[string]string
	message    string
}

func (c *command) String() string {
	kind := c.command
	if kind == "" {
		// same placeholder as @actions/core, so the runner reports a recognisable command
		kind = "missing.command"
	}
	s := cmdString + kind
	sep := " "
	for _, key := range sortedPropertyKeys(c.properties) {
		s += sep + key + "=" + escape(c.properties[key])
		sep = ","
	}
	return s + cmdString + escapeData(c.message)
}

// propertyOrder is the order @actions/core emits annotation properties in.
var propertyOrder = []string{"title", "file", "line", "endLine", "col", "endColumn"}

// sortedPropertyKeys returns the keys in a deterministic order: the annotation
// properties in the order @actions/core uses, then any other key alphabetically.
func sortedPropertyKeys(properties map[string]string) []string {
	keys := make([]string, 0, len(properties))
	for _, k := range propertyOrder {
		if _, ok := properties[k]; ok {
			keys = append(keys, k)
		}
	}
	var others []string
	for k := range properties {
		if !contains(propertyOrder, k) {
			others = append(others, k)
		}
	}
	sort.Strings(others)
	return append(keys, others...)
}

func contains(list []string, v string) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}

func escapePatterns(v string, replacementsArg ...map[string]string) string {
	v = strings.Replace(v, "%", "%25", -1)
	for _, replacements := range replacementsArg {
		for pattern, replacement := range replacements {
			v = strings.Replace(v, pattern, replacement, -1)
		}
	}
	return v
}

func escapeData(v string) string {
	return escapePatterns(v, dataEscapes)
}

func escape(v string) string {
	return escapePatterns(v, escapes, dataEscapes)
}
