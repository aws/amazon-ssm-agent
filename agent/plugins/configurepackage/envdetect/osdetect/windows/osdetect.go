package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
)

type Detector struct {
}

// https://msdn.microsoft.com/en-us/library/aa394239%28v=vs.85%29.aspx

func (*Detector) DetectPkgManager(platform string, version string, family string) (string, error) {
	return c.PackageManagerWindows, nil
}

func (*Detector) DetectInitSystem() (string, error) {
	return c.InitWindows, nil
}

func (*Detector) DetectPlatform() (string, string, string, error) {
	output, err := getWmiOSInfo()
	if err != nil {
		return c.PlatformWindows, "", c.PlatformFamilyWindows, nil
	}

	version, err := parseVersion(output)

	// TODO: differentiate between normal and nano server? -> SKU
	// OperatingSystemSKU

	// TODO: get full version
	// CSDVersion
	// BuildNumber
	//fullVersion := fmt.Sprintf("%s %s %s", version, csdVersion, buildNumber)

	// TODO: name as platform?
	// Caption

	return c.PlatformWindows, version, c.PlatformFamilyWindows, err
}

func getWmiOSInfo() (string, error) {
	wmicCommand := filepath.Join(os.Getenv("WINDIR"), "System32", "wbem", "wmic.exe")
	cmdArgs := []string{"OS", "get", "/format:list"}
	cmdOut, err := exec.Command(wmicCommand, cmdArgs...).Output()
	if err != nil {
		return "", err
	}

	return string(cmdOut), err
}

func parseVersion(wmioutput string) (string, error) {
	return parseProperty(wmioutput, `(?m)^\s*Version\s*=\s*(.+\S)\s*$`)
}

func parseProperty(wmioutput string, regex string) (string, error) {
	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(wmioutput)

	if len(match) > 0 {
		return match[1], nil
	}

	return "", fmt.Errorf("could not parse windows version")
}
