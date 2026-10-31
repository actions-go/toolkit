//go:build !windows

package cache

import (
	"os"
	"syscall"
)

// umask returns the umask of the process.
func umask() os.FileMode {
	m := syscall.Umask(0)
	syscall.Umask(m)
	return os.FileMode(m)
}
