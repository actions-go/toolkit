package core

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	// StatusFailed is returned by Status() in case this action has been marked as failed
	StatusFailed = 1
	// StatusSuccess is returned by Status() in case this action has not been marked as failed. By default an action is claimed as successful
	StatusSuccess = 0
)

var (
	status       = StatusSuccess
	statusAccess = &sync.Mutex{}
)

// ExportVariable sets the environment varaible name (for this action and future actions)
func ExportVariable(name, value string) {
	os.Setenv(name, value)
	IssueCommand("set-env", map[string]string{"name": name}, value)
}

// SetSecret registers a secret which will get masked from logs
func SetSecret(secret string) {
	Issue("add-mask", secret)
}

// AddPath prepends inputPath to the PATH (for this action and future actions)
func AddPath(path string) {
	// TODO js: process.env['PATH'] = `${inputPath}${path.delimiter}${process.env['PATH']}`
	Issue("add-path", path)
}

// GetInput gets the value of an input.  The value is also trimmed.
func GetInput(name string) (string, bool) {
	val, ok := os.LookupEnv(strings.ToUpper("INPUT_" + strings.Replace(name, " ", "_", -1)))
	return strings.TrimSpace(val), ok
}

// SetOutput sets the value of an output for future actions
func SetOutput(name, value string) {
	IssueCommand("set-output", map[string]string{"name": name}, value)
}

// SetFailed sets the action status to failed and sets an error message
func SetFailed(message string) {
	statusAccess.Lock()
	status = StatusFailed
	statusAccess.Unlock()
	Error(message)
}

// Debug writes debug message to user log
func Debug(message string) {
	Issue("debug", message)
}

// Debugf writes debug message to user log
func Debugf(format string, args ...interface{}) {
	Debug(fmt.Sprintf(format, args...))
}

// Error adds an error issue
func Error(message string) {
	Issue("error", message)
}

// Errorf writes debug message to user log
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Warning adds a warning issue
func Warning(message string) {
	Issue("warning", message)
}

// Warningf writes debug message to user log
func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

// Info writes the message on the console
func Info(message string) {
	fmt.Println(message)
}

// Infof writes debug message to user log
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// StartGroup begin an output group. Output until the next `GroupEnd` will be foldable in this group
func StartGroup(name string) {
	Issue("group", name)
}

// EndGroup end an output group and folds it
func EndGroup() {
	Issue("endgroup")
}

// Group wrap an asynchronous function call in a group, all logs of the function will be collapsed after completion
func Group(name string, f func()) func() {
	return func() {
		StartGroup(name)
		defer EndGroup()
		f()
	}
}

// SaveState saves state for current action, the state can only be retrieved by this action's post job execution.
func SaveState(name, value string) {
	IssueCommand("save-state", map[string]string{"name": name}, value)
}

// GetState gets the value of an state set by this action's main execution.
func GetState(name string) string {
	return os.Getenv("STATE_" + name)
}
