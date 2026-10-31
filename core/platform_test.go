package core

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformConstants(t *testing.T) {
	assert.Equal(t, nodePlatform(runtime.GOOS), Platform)
	assert.Equal(t, runtime.GOOS == "windows", IsWindows)
	assert.Equal(t, runtime.GOOS == "darwin", IsMacOS)
	assert.Equal(t, runtime.GOOS == "linux", IsLinux)
}

// the expected names are the values of Node.js os.platform()
func TestNodePlatformNames(t *testing.T) {
	assert.Equal(t, "win32", nodePlatform("windows"))
	assert.Equal(t, "sunos", nodePlatform("solaris"))
	assert.Equal(t, "linux", nodePlatform("linux"))
	assert.Equal(t, "darwin", nodePlatform("darwin"))
}

// the expected names are the values of Node.js os.arch()
func TestNodeArchNames(t *testing.T) {
	for goarch, want := range map[string]string{
		"amd64": "x64", "386": "ia32", "arm64": "arm64", "arm": "arm",
		"mipsle": "mipsel", "ppc64le": "ppc64", "riscv64": "riscv64", "s390x": "s390x",
	} {
		assert.Equal(t, want, nodeArch(goarch), goarch)
	}
	assert.Equal(t, nodeArch(runtime.GOARCH), Arch)
}

func TestRunCommandReportsMissingExecutableLikeTheReference(t *testing.T) {
	_, err := runCommand("parity-no-such-executable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unable to locate executable file: parity-no-such-executable.")
}

func TestGetDetailsStructure(t *testing.T) {
	origRunCmd := runCommand
	t.Cleanup(func() { runCommand = origRunCmd })

	// Stub out OS command execution for non-native platforms.
	switch runtime.GOOS {
	case "darwin":
		runCommand = func(name string, args ...string) (string, error) {
			return "ProductName:    macOS\nProductVersion: 13.0\nBuildVersion:   22A380\n", nil
		}
	case "linux":
		runCommand = func(name string, args ...string) (string, error) {
			return "Ubuntu\n22.04\n", nil
		}
	case "windows":
		runCommand = func(name string, args ...string) (string, error) {
			// Return version for first call, name for second call.
			return "10.0.19041\n", nil
		}
	}

	details, err := GetDetails()
	require.NoError(t, err)

	assert.Equal(t, Platform, details.Platform)
	assert.Equal(t, Arch, details.Arch)
	assert.Equal(t, IsWindows, details.IsWindows)
	assert.Equal(t, IsMacOS, details.IsMacOS)
	assert.Equal(t, IsLinux, details.IsLinux)
	assert.NotEmpty(t, details.Platform)
	assert.NotEmpty(t, details.Arch)
}

func TestGetDetailsMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}
	origRunCmd := runCommand
	t.Cleanup(func() { runCommand = origRunCmd })

	runCommand = func(name string, args ...string) (string, error) {
		return "ProductName:    macOS\nProductVersion: 13.5.0\nBuildVersion:   22G74\n", nil
	}

	details, err := GetDetails()
	require.NoError(t, err)
	assert.Equal(t, "macOS", details.Name)
	assert.Equal(t, "13.5.0", details.Version)
}

func TestGetDetailsLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	origRunCmd := runCommand
	t.Cleanup(func() { runCommand = origRunCmd })

	runCommand = func(name string, args ...string) (string, error) {
		return "Ubuntu\n22.04\n", nil
	}

	details, err := GetDetails()
	require.NoError(t, err)
	assert.Equal(t, "Ubuntu", details.Name)
	assert.Equal(t, "22.04", details.Version)
}

func TestGetDetailsWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	origRunCmd := runCommand
	t.Cleanup(func() { runCommand = origRunCmd })

	callCount := 0
	runCommand = func(name string, args ...string) (string, error) {
		callCount++
		if callCount == 1 {
			return "10.0.19041\n", nil // version
		}
		return "Microsoft Windows 10 Pro\n", nil // name
	}

	details, err := GetDetails()
	require.NoError(t, err)
	assert.Equal(t, "Microsoft Windows 10 Pro", details.Name)
	assert.Equal(t, "10.0.19041", details.Version)
}
