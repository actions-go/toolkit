package cache

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// hostOS is the operating system the extractors that only exist on one of
// them check, like IS_WINDOWS and IS_MAC of @actions/tool-cache. It is a
// variable so that tests can run the code of another system with a fake
// binary.
var hostOS = runtime.GOOS

// runTool runs tool the way @actions/exec does for extract7z and extractXar:
// the command line is echoed unless silent, the output of the tool goes to
// the output of the process (nothing when silent) and a failure code is the
// error "The process ... failed with exit code N". The tool never reads the
// standard input.
func runTool(tool string, args []string, dir string, silent bool) error {
	if !silent {
		echoCommand(tool, args)
	}
	cmd := exec.Command(tool, args...)
	cmd.Dir = dir
	if silent {
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	} else {
		// @actions/exec writes the tool's stderr into the same log stream as
		// its stdout, so a failing tool's message appears in the step log
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stdout
	}
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return processError(tool, exit.ExitCode())
	}
	return err
}
