package cache

import (
	"errors"

	"github.com/actions-go/toolkit/core"
)

// Extract7z extracts a .7z archive into dest, a new folder below RUNNER_TEMP
// when dest is empty, and returns the destination, like extract7z of
// @actions/tool-cache. It is only supported on Windows.
//
// sevenZipPath is the 7zr.exe (or 7z.exe) to run, as "x -bb0 -bd -sccUTF-8
// <file>" in dest (-bb1 when debugging is on), silently. The reference runs
// the 7zdec.exe it bundles when no path is given; this package bundles
// nothing, so it looks for 7z, then 7zr, in PATH instead.
func Extract7z(file, dest, sevenZipPath string) (string, error) {
	if hostOS != "windows" {
		return "", errors.New("extract7z() not supported on current OS")
	}
	if file == "" {
		return "", errors.New(`parameter "file" is required`)
	}
	dest, err := createExtractFolder(dest)
	if err != nil {
		return "", err
	}
	tool := sevenZipPath
	if tool == "" {
		if tool, err = which("7z"); err != nil {
			if tool, err = which("7zr"); err != nil {
				return "", err
			}
		}
	}
	logLevel := "-bb0"
	if core.IsDebug() {
		logLevel = "-bb1"
	}
	// the file is relative to dest: the reference changes the directory first
	if err := runTool(tool, []string{"x", logLevel, "-bd", "-sccUTF-8", file}, dest, true); err != nil {
		return "", err
	}
	return dest, nil
}
