package darwin

import (
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
)

type Detector struct {
}

func (*Detector) DetectPkgManager(platform string, version string, family string) (string, error) {
	return c.PackageManagerMac, nil
}

func (*Detector) DetectInitSystem() (string, error) {
	return c.InitLaunchd, nil
}

func (*Detector) DetectPlatform(_ log.T) (string, string, string, error) {
	cmdOut, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil {
		return "", "", "", err
	}

	return c.PlatformDarwin, extractDarwinVersion(cmdOut), c.PlatformFamilyDarwin, nil
}

func extractDarwinVersion(data []byte) string {
	return strings.TrimSpace(string(data))
}
