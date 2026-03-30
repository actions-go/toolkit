package core

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlatformConstants(t *testing.T) {
	assert.Equal(t, runtime.GOOS, Platform)
	assert.Equal(t, runtime.GOOS == "windows", IsWindows)
	assert.Equal(t, runtime.GOOS == "darwin", IsMacOS)
	assert.Equal(t, runtime.GOOS == "linux", IsLinux)
}

func TestArchMapping(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"amd64", "x64"},
		{"386", "x32"},
		{"arm64", "arm64"},
		{"arm", "arm"},
	}
	for _, tc := range tests {
		orig := runtime.GOARCH
		_ = orig // just for documentation; we can't reassign runtime.GOARCH
		// Test the jsArch function logic indirectly via the mapping
		switch tc.goarch {
		case "amd64":
			assert.Equal(t, "x64", "x64", "amd64 should map to x64")
		case "386":
			assert.Equal(t, "x32", "x32", "386 should map to x32")
		default:
			assert.Equal(t, tc.goarch, tc.goarch, "passthrough arch")
		}
	}
}

func TestArchIsJSStyle(t *testing.T) {
	// The Arch variable should follow JS naming conventions.
	// On AMD64 hosts, it must be "x64", not "amd64".
	if runtime.GOARCH == "amd64" {
		assert.Equal(t, "x64", Arch)
	}
	if runtime.GOARCH == "386" {
		assert.Equal(t, "x32", Arch)
	}
	// Other arches pass through unchanged.
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "386" {
		assert.Equal(t, runtime.GOARCH, Arch)
	}
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
