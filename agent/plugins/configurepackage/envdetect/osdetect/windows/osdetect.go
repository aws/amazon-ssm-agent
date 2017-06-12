package windows

import (
	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
)

type Detector struct {
}

func (*Detector) DetectPkgManager(platform string, version string, family string) (string, error) {
	return c.PackageManagerWindows, nil
}

func (*Detector) DetectInitSystem() (string, error) {
	return c.InitWindows, nil
}

func (*Detector) DetectPlatform() (string, string, string, error) {
	return c.PlatformWindows, "TODO", c.PlatformFamilyWindows, nil
	// TODO: detect windows version
	// TODO: differentiate between normal and nano server?
}
