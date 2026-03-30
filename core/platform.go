package core

import (
	"os/exec"
	"runtime"
	"strings"
)

// Platform is the current operating system platform (e.g. "linux", "darwin", "windows").
// Mirrors os.platform() from Node.js.
var Platform = runtime.GOOS

// Arch is the current CPU architecture (e.g. "x64", "arm64").
// Mirrors os.arch() from Node.js — uses JavaScript naming conventions.
var Arch = jsArch()

// IsWindows reports whether the current platform is Windows.
var IsWindows = runtime.GOOS == "windows"

// IsMacOS reports whether the current platform is macOS.
var IsMacOS = runtime.GOOS == "darwin"

// IsLinux reports whether the current platform is Linux.
var IsLinux = runtime.GOOS == "linux"

// PlatformDetails contains detailed information about the current operating system.
type PlatformDetails struct {
	// Name is the OS display name (e.g. "Ubuntu 22.04.1 LTS", "macOS 13.0").
	Name string
	// Platform is the OS platform identifier (e.g. "linux", "darwin", "windows").
	Platform string
	// Arch is the CPU architecture using JavaScript naming (e.g. "x64", "arm64").
	Arch string
	// Version is the OS version string.
	Version string
	// IsWindows reports whether the platform is Windows.
	IsWindows bool
	// IsMacOS reports whether the platform is macOS.
	IsMacOS bool
	// IsLinux reports whether the platform is Linux.
	IsLinux bool
}

// GetDetails returns detailed platform information including OS name and version.
// It executes OS-specific commands to retrieve this information.
func GetDetails() (PlatformDetails, error) {
	details := PlatformDetails{
		Platform:  Platform,
		Arch:      Arch,
		IsWindows: IsWindows,
		IsMacOS:   IsMacOS,
		IsLinux:   IsLinux,
	}

	var err error
	if IsWindows {
		details.Name, details.Version, err = getWindowsInfo()
	} else if IsMacOS {
		details.Name, details.Version, err = getMacOsInfo()
	} else {
		details.Name, details.Version, err = getLinuxInfo()
	}
	return details, err
}

func getWindowsInfo() (name, version string, err error) {
	versionOut, err := runCommand("powershell", "-command", "(Get-CimInstance -ClassName Win32_OperatingSystem).Version")
	if err != nil {
		return "", "", err
	}
	nameOut, err := runCommand("powershell", "-command", "(Get-CimInstance -ClassName Win32_OperatingSystem).Caption")
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(nameOut), strings.TrimSpace(versionOut), nil
}

func getMacOsInfo() (name, version string, err error) {
	out, err := runCommand("sw_vers")
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "ProductVersion:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "ProductVersion:"))
		} else if strings.HasPrefix(line, "ProductName:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "ProductName:"))
		}
	}
	return name, version, nil
}

func getLinuxInfo() (name, version string, err error) {
	out, err := runCommand("lsb_release", "-i", "-r", "-s")
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\n", 2)
	if len(parts) > 0 {
		name = parts[0]
	}
	if len(parts) > 1 {
		version = parts[1]
	}
	return name, version, nil
}

// jsArch maps Go's GOARCH to Node.js os.arch() naming conventions.
func jsArch() string {
	switch runtime.GOARCH {
	case "386":
		return "x32"
	case "amd64":
		return "x64"
	default:
		return runtime.GOARCH
	}
}

// runCommand executes a command and returns its combined stdout output.
var runCommand = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
