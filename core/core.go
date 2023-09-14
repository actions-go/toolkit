package core

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

const (
	delimiter = "_GitHubActionsGoFileCommandDelimeter_"

	// StatusFailed is returned by Status() in case this action has been marked as failed
	StatusFailed = 1
	// StatusSuccess is returned by Status() in case this action has not been marked as failed. By default an action is claimed as successful
	StatusSuccess = 0

	GitHubOutputFilePathEnvName    = "GITHUB_OUTPUT"
	GitHubStateFilePathEnvName     = "GITHUB_STATE"
	GitHubExportEnvFilePathEnvName = "GITHUB_ENV"
	GitHubPathFilePathEnvName      = "GITHUB_PATH"
)

var (
	status       = StatusSuccess
	statusAccess = &sync.Mutex{}
	lookupEnv    = os.LookupEnv
	open         = func(path string, flag int, perm os.FileMode) (File, error) {
		fd, err := os.OpenFile(path, flag, perm)
		if err != nil {
			return nil, err
		}
		return fd, nil
	}
)

type File interface {
	io.Reader
	io.Writer
	io.Closer
}

func formatOutput(name, value string) string {
	return strings.Join(
		[]string{
			fmt.Sprintf("%s<<%s", name, delimiter),
			value,
			delimiter,
			"",
		},
		EOF,
	)
}

// ExportVariable sets the environment varaible name (for this action and future actions)
func ExportVariable(name, value string) {
	if err := issueFileCommand(GitHubExportEnvFilePathEnvName, formatOutput(name, value)); err != nil {
		IssueCommand("set-env", map[string]string{"name": name}, value)
	}
	os.Setenv(name, value)
}

// SetSecret registers a secret which will get masked from logs
func SetSecret(secret string) {
	Issue("add-mask", secret)
}

// AddPath prepends inputPath to the PATH (for this action and future actions)
func AddPath(path string) {
	if err := issueFileCommand(GitHubPathFilePathEnvName, path); err != nil {
		Issue("add-path", path)
	}
	// TODO js: process.env['PATH'] = `${inputPath}${path.delimiter}${process.env['PATH']}`
}

// GetBoolInput gets the value of an input and returns whether it equals "true".
// In any other case, whether it does not equal, or the input is not set, false is returned
func GetBoolInput(name string) bool {
	return strings.ToLower(GetInputOrDefault(name, "false")) == "true"
}

// GetInput gets the value of an input.  The value is also trimmed.
func GetInput(name string) (string, bool) {
	val, ok := lookupEnv(strings.ToUpper("INPUT_" + strings.Replace(name, " ", "_", -1)))
	return strings.TrimSpace(val), ok
}

// GetInputOrDefault gets the value of an input. If value is not found, a default value is used
func GetInputOrDefault(name, dflt string) string {
	val, ok := GetInput(name)
	if ok {
		return val
	}
	return dflt
}

// SetOutput sets the value of an output for future actions
func SetOutput(name, value string) {
	if err := issueFileCommand(GitHubOutputFilePathEnvName, formatOutput(name, value)); err != nil {
		Warningf("did not find output file from environment variable %s, falling back to the deprecated command implementation", GitHubOutputFilePathEnvName)
		IssueCommand("set-output", map[string]string{"name": name}, value)
	}
}

// SetFailedf sets the action status to failed and sets an error message
func SetFailedf(format string, args ...interface{}) {
	SetFailed(fmt.Sprintf(format, args...))
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

// StopCommands Stops processing any workflow commands.
// Commands will be resumed when calling StartCommands(endToken)
// This special command allows you to log anything without accidentally running a workflow command.
// For example, you could stop logging to output an entire script that has comments.
func StopCommands(endToken string) {
	Issue("stop-commands", endToken)
}

// StartCommands enables commands stopped until the endToken
func StartCommands(endToken string) {
	Issue(endToken)
}

// WithoutCommands executes the functions ensuring it does not execute any github actions commands.
// This special command allows you to log anything without accidentally running a workflow command.
// For example, you could stop logging to output an entire script that has comments.
func WithoutCommands(endToken string, f func()) {
	StopCommands(endToken)
	defer StartCommands(endToken)
	f()
}

// SaveState saves state for current action, the state can only be retrieved by this action's post job execution.
func SaveState(name, value string) {
	if err := issueFileCommand(GitHubStateFilePathEnvName, formatOutput(name, value)); err != nil {
		Warningf("did not find state file from environment variable %s, falling back to the deprecated command implementation", GitHubStateFilePathEnvName)
		IssueCommand("save-state", map[string]string{"name": name}, value)
	}
}

// GetState gets the value of an state set by this action's main execution.
func GetState(name string) string {
	return os.Getenv("STATE_" + name)
}

// IsDebug returns whether the github actions is currently under debug
func IsDebug() bool {
	return os.Getenv("RUNNER_DEBUG") == "1"
}
