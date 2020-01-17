package core

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	cmdString = "::"
)

var (
	stdout       io.Writer = os.Stdout
	stdoutSetter sync.Mutex
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

type command struct {
	command    string
	properties map[string]string
	message    string
}

func (c *command) String() string {
	s := cmdString + c.command
	sep := " "
	for key, value := range c.properties {
		s += sep + key + "=" + escape(value)
		sep = ","
	}
	return s + cmdString + escape(c.message)
}

func escapeData(v string) string {
	return strings.Replace(
		strings.Replace(
			v,
			"\r", "%0D", -1,
		),
		"\n", "%0A", -1,
	)
}

func escape(v string) string {
	return strings.Replace(
		strings.Replace(
		strings.Replace(
			escapeData(v),
			"]", "%5D", -1,
		),
		";", "%3B", -1,
	),
	",", "%2C",-1,
)
}
