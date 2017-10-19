package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/log"

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

func (*Detector) DetectPlatform(log log.T) (string, string, string, error) {
	output, err := getWmiOSInfo()
	if err != nil {
		log.Infof("Could not retrieve WmiOSInfo, proceeding without 'Version' - %v", err)
		return c.PlatformWindows, "", c.PlatformFamilyWindows, nil
	}
	return detectPlatformDetails(log, output)
}

func detectPlatformDetails(log log.T, wmioutput string) (string, string, string, error) {
	osSKU, nonFatalErr := parseOperatingSystemSKU(wmioutput)
	if nonFatalErr != nil {
		log.Infof("Proceeding without knowing OperatingSystemSKU - %v", nonFatalErr)
	}

	version, err := parseVersion(wmioutput)
	if isWindowsNano(osSKU) {
		version = fmt.Sprint(version, "nano")
	}

	// TODO: get full version
	// CSDVersion
	// BuildNumber
	//fullVersion := fmt.Sprintf("%s %s %s", version, csdVersion, buildNumber)

	// TODO: name as platform?
	// Caption

	return c.PlatformWindows, version, c.PlatformFamilyWindows, err
}

func isWindowsNano(operatingSystemSKU string) bool {
	return operatingSystemSKU == c.SKUProductStandardNanoServer ||
		operatingSystemSKU == c.SKUProductDatacenterNanoServer
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
	return parseProperty(wmioutput, "Version")
}

func parseOperatingSystemSKU(wmioutput string) (string, error) {
	return parseProperty(wmioutput, "OperatingSystemSKU")
}

func parseProperty(wmioutput string, property string) (string, error) {
	regex := fmt.Sprintf(`(?m)^\s*%s\s*=\s*(\S+)\s*$`, property)
	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(wmioutput)

	if len(match) > 0 {
		return match[1], nil
	}

	return "", fmt.Errorf("could not parse wmi property '%s'", property)
}
