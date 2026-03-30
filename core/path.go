package core

import (
	"path/filepath"
	"strings"
)

// ToPosixPath converts the given path to POSIX form. On Windows, backslashes are replaced with forward slashes.
func ToPosixPath(pth string) string {
	return strings.ReplaceAll(pth, `\`, "/")
}

// ToWin32Path converts the given path to Windows form. Forward slashes are replaced with backslashes.
func ToWin32Path(pth string) string {
	return strings.ReplaceAll(pth, "/", `\`)
}

// ToPlatformPath converts the given path to the platform-specific form using the OS path separator.
func ToPlatformPath(pth string) string {
	return strings.NewReplacer("/", string(filepath.Separator), `\`, string(filepath.Separator)).Replace(pth)
}
