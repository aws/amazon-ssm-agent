// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// RegionHolder represents Place holder for Region
	RegionHolder = "{Region}"

	// ComponentNameHolder represents Place holder for component name
	ComponentNameHolder = "{ComponentName}"

	// PackageVersionHolder represents Place holder for package version
	PackageVersionHolder = "{PackageVersion}"

	// PackageNameHolder represents Place holder for package name
	PackageNameHolder = "{PackageName}"

	// PlatformHolder represents Place holder for platform
	PlatformHolder = "{Platform}"

	// ArchHolder represents Place holder for architecture
	ArchHolder = "{Arch}"

	// CompressedHolder represents Place holder for compress format
	CompressedHolder = "{Compressed}"

	PlatformWindows     = "Windows"
	PlatformWindowsNano = "WindowsNano"
	PlatformLinux       = "Linux"

	WindowsExtension = "zip"
	LinuxExtension   = "tar.gz"
)

type Util interface {
	CreateComponentFolder(input *ConfigureComponentPluginInput) (folder string, err error)
}

type Utility struct{}

// CreatePackageName constructs the package name to locate in the s3 bucket
// Assumes valid non-empty input name, architecture, and platform
// TO DO: implement validate plugin input function to assert above assumption
func CreatePackageName(input *ConfigureComponentPluginInput) (packageName string) {
	// file name format based on agreed convention
	packageName = "{ComponentName}-{Arch}.{Compressed}"

	packageName = strings.Replace(packageName, ComponentNameHolder, input.Name, -1)
	packageName = strings.Replace(packageName, ArchHolder, input.Architecture, -1)

	// file name extension based on platform type
	if input.Platform == PlatformWindows || input.Platform == PlatformWindowsNano {
		packageName = strings.Replace(packageName, CompressedHolder, WindowsExtension, -1)
	} else if input.Platform == PlatformLinux {
		packageName = strings.Replace(packageName, CompressedHolder, LinuxExtension, -1)
	}
	return packageName
}

// CreatePackageLocation constructs the s3 url to locate the package for downloading
func CreatePackageLocation(input *ConfigureComponentPluginInput, context *updateutil.InstanceContext, packageName string) (packageLocation string) {
	// s3 uri format based on agreed convention
	packageLocation = "https://amazon-ssm-{Region}.s3.amazonaws.com/{ComponentName}/{Platform}/{PackageVersion}/{PackageName}"

	packageLocation = strings.Replace(packageLocation, RegionHolder, context.Region, -1)
	packageLocation = strings.Replace(packageLocation, ComponentNameHolder, input.Name, -1)
	packageLocation = strings.Replace(packageLocation, PlatformHolder, input.Platform, -1)
	packageLocation = strings.Replace(packageLocation, PackageVersionHolder, input.Version, -1)
	packageLocation = strings.Replace(packageLocation, PackageNameHolder, packageName, -1)

	return packageLocation
}

var mkDirAll = os.MkdirAll

// CreateComponentFolder constructs the local directory to place component
func (util *Utility) CreateComponentFolder(input *ConfigureComponentPluginInput) (folder string, err error) {
	folder = filepath.Join(appconfig.DownloadRoot, "components", input.Name, input.Version)
	if err = mkDirAll(folder, os.ModePerm|os.ModeDir); err != nil {
		return "", err
	}

	return folder, nil
}
