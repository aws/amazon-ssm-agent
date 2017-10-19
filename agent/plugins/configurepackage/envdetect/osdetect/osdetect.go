package osdetect

import (
	"fmt"
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect/darwin"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect/linux"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect/windows"
)

type OsDetector interface {
	DetectPlatform(log.T) (string, string, string, error)
	DetectInitSystem() (string, error)
	DetectPkgManager(string, string, string) (string, error)
}

// OperatingSystem contains operating system information and capabilities
// Identifies are aligned with Ohai data naming.
type OperatingSystem struct {
	Platform        string
	PlatformVersion string
	PlatformFamily  string
	Architecture    string
	InitSystem      string
	PackageManager  string
}

// CollectOSData quires the operating system for type and capabilities
func CollectOSData(log log.T) (*OperatingSystem, error) {
	var d OsDetector
	switch runtime.GOOS {
	case "darwin":
		d = &darwin.Detector{}
	case "linux":
		d = &linux.Detector{}
	case "windows":
		d = &windows.Detector{}
	default:
		return nil, fmt.Errorf("unknown platform: %s", runtime.GOOS)
	}

	platform, platformVersion, platformFamily, err := d.DetectPlatform(log)
	if err != nil {
		return nil, err
	}

	init, err := d.DetectInitSystem()
	if err != nil {
		return nil, err
	}

	pkg, err := d.DetectPkgManager(platform, platformVersion, platformFamily)
	if err != nil {
		return nil, err
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}

	e := &OperatingSystem{
		Platform:        platform,
		PlatformVersion: platformVersion,
		PlatformFamily:  platformFamily,
		Architecture:    arch,
		InitSystem:      init,
		PackageManager:  pkg,
	}
	return e, err
}
