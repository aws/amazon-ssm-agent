package linux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/utils"
)

type Detector struct {
}

func (*Detector) DetectPkgManager(platform string, version string, family string) (string, error) {
	switch family {
	case c.PlatformFamilyDebian:
		return c.PackageManagerApt, nil
	case c.PlatformFamilyAlpine:
		return c.PackageManagerAlpine, nil
	case c.PlatformFamilyArch:
		return c.PackageManagerPacman, nil
	case c.PlatformFamilySuse:
		return c.PackageManagerZipper, nil
	case c.PlatformFamilyGentoo:
		return c.PackageManagerEmerge, nil
	case c.PlatformFamilyFedora:
		return c.PackageManagerDnf, nil
	case c.PlatformFamilyRhel:
		return c.PackageManagerYum, nil
	default:
		return "", fmt.Errorf("could not detect package manager for: `%s`, `%s`, `%s`", platform, version, family)
	}
}

func (*Detector) DetectInitSystem() (string, error) {
	var cmdOut []byte
	var err error
	var data string

	data, err = utils.ReadFileTrim("/proc/1/comm")
	if err == nil && strings.Contains(strings.ToLower(data), "systemd") {
		return data, nil
	}

	cmdOut, err = exec.Command("ps", "-1").Output()
	if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "systemd") {
		return c.InitSystemd, nil
	}

	data, err = utils.ReadFileTrim("/proc/1/cgroup")
	if err == nil && strings.Contains(strings.ToLower(data), "docker") {
		return c.InitDocker, nil
	}

	cmdOut, err = exec.Command("service", "--version").Output()
	if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "systemd") {
		return c.InitSystemd, nil
	} else if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "openrc") {
		return c.InitOpenrc, nil
	}

	cmdOut, err = exec.Command("/sbin/initctl", "--version").Output()
	if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "upstart") {
		return c.InitUpstart, nil
	}

	// systemctl might be in /bin, /usr/bin
	cmdOut, err = exec.Command("/bin/systemctl", "--version").Output()
	if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "systemd") {
		return c.InitSystemd, nil
	}
	cmdOut, err = exec.Command("/usr/bin/systemctl", "--version").Output()
	if err == nil && strings.Contains(strings.ToLower(string(cmdOut)), "systemd") {
		return c.InitSystemd, nil
	}

	// systems which have update-rc.d might also have chkconfig but rarely the
	// other way round - so checking update-rc.d first
	_, err = exec.Command("/sbin/update-rc.d").Output()
	if err == nil {
		return c.InitUpdatercd, nil
	}

	// chkconfig does not return any useful help/version
	_, err = exec.Command("/sbin/chkconfig").Output()
	if err == nil {
		return c.InitChkconfig, nil
	}
	_, err = exec.Command("/usr/bin/chkconfig").Output()
	if err == nil {
		return c.InitChkconfig, nil
	}

	return "", errors.New("could not determine init system")
}

func (*Detector) DetectPlatform(_ log.T) (string, string, string, error) {
	var platform, version, platformFamily string
	var err error

	platform, version, err = scanOSrelease()
	if err != nil {
		platform, version, err = scanLSB()
		if err != nil {
			platform, version, err = scanDistributionReleaseFiles()
		}
	}

	if err != nil {
		return "", "", "", nil
	}

	platformFamily, err = platformFamilyForPlatform(platform)

	return platform, version, platformFamily, err
}

///////////////////////////
// /etc/os-release
// http://0pointer.de/blog/projects/os-release.html
// http://0pointer.de/public/systemd-man/os-release.html
///////////////////////////

func scanOSrelease() (string, string, error) {
	var lines []string
	var err error

	if _, err = os.Stat("/etc/os-release"); err == nil {
		lines, err = utils.ReadFileLines("/etc/os-release")
	} else if _, err = os.Stat("/usr/lib/os-release"); err == nil {
		lines, err = utils.ReadFileLines("/usr/lib/os-release")
	} else {
		err = errors.New("no os-release file found")
	}

	if err != nil {
		return "", "", err
	}

	return parseOSreleaseFile(lines)
}

func parseOSreleaseFile(lines []string) (string, string, error) {
	var platform, platformVersion, name string
	// TODO: Shell special characters ("$", quotes, backslash, backtick) must be escaped with backslashes
	NameRexp := regexp.MustCompile(`^NAME=["']?(.+?)["']?\n?$`)
	platformRexp := regexp.MustCompile(`^ID=["']?(.+?)["']?\n?$`)
	platformVersionRexp := regexp.MustCompile(`^VERSION_ID=["']?(.+?)["']?\n?$`)

	for _, line := range lines {
		match := platformRexp.FindStringSubmatch(line)
		if len(match) > 0 {
			platform = match[1]
		}

		match = platformVersionRexp.FindStringSubmatch(line)
		if len(match) > 0 {
			platformVersion = match[1]
		}

		match = NameRexp.FindStringSubmatch(line)
		if len(match) > 0 {
			name = match[1]
		}
	}

	if platform == "" && platformVersion == "" {
		return "", "", fmt.Errorf("could not find platform information in os-release: %v", lines)
	}

	switch platform {
	case "rhel":
		platform = c.PlatformRedhat
	case "amzn":
		platform = c.PlatformAmazon
	case "sles", "suse":
		platform = c.PlatformSuse
	case "opensuse":
		if strings.Contains(strings.ToLower(name), "leap") {
			platform = c.PlatformOpensuseLeap
		}
	}

	return platform, platformVersion, nil
}

///////////////////////////
// man lsb_release
//
// via command:
//
//   $ ./lsb_release --all
//   Distributor ID:	Ubuntu
//   Description:	Ubuntu Zesty Zapus (development branch)
//   Release:	17.04
//   Codename:	zesty
//
// via file:
//
//   $ cat /etc/lsb-release
//   DISTRIB_ID=Ubuntu
//   DISTRIB_RELEASE=17.04
//   DISTRIB_CODENAME=zesty
//   DISTRIB_DESCRIPTION="Ubuntu Zesty Zapus (development branch)"
///////////////////////////

func scanLSB() (string, string, error) {
	var platform, platformVersion string
	var err error

	platform, platformVersion, err = scanLSBreleaseCMD()

	if err != nil {
		// If lsb-release command is not available try reading lsb file
		platform, platformVersion, err = scanLSBreleaseFile()
		if err != nil {
			return "", "", err
		}
	}

	return platform, platformVersion, nil
}

func scanLSBreleaseCMD() (string, string, error) {
	cmdOutID, err := exec.Command("lsb_release", "--short", "--id").Output()
	if err != nil {
		return "", "", err
	}

	cmdOutRelease, err := exec.Command("lsb_release", "--short", "--release").Output()
	if err != nil {
		return "", "", err
	}

	platform, version := parseLSBreleaseCMD(cmdOutID, cmdOutRelease)
	return platform, version, nil
}

func parseLSBreleaseCMD(id []byte, release []byte) (string, string) {
	var platform, platformVersion string

	platform = strings.ToLower(strings.TrimSpace(string(id)))

	// substitutions to match expected ohai identifier
	platformSubstitutions := map[string]string{
		"redhatenterpriseserver": c.PlatformRedhat,
		"suse linux":             c.PlatformSuse,
		"opensuse project":       c.PlatformOpensuse,
	}

	if result, ok := platformSubstitutions[platform]; ok {
		platform = result // overwrite platform with substitution
	}

	platformVersion = strings.ToLower(strings.TrimSpace(string(release)))

	return platform, platformVersion
}

func scanLSBreleaseFile() (string, string, error) {
	lines, err := utils.ReadFileLines("/etc/lsb-release")
	if err != nil {
		return "", "", err
	}
	return parseLSBreleaseFile(lines)
}

func parseLSBreleaseFile(lines []string) (string, string, error) {
	var platform, platformVersion string
	platformRexp := regexp.MustCompile(`^DISTRIB_ID=["']?(.+?)["']?\n?$`)
	platformVersionRexp := regexp.MustCompile(`^DISTRIB_RELEASE=["']?(.+?)["']?\n?$`)

	for _, line := range lines {
		match := platformRexp.FindStringSubmatch(line)
		if len(match) > 0 {
			platform = match[1]
		}

		match = platformVersionRexp.FindStringSubmatch(line)
		if len(match) > 0 {
			platformVersion = match[1]
		}
	}

	if platform == "" && platformVersion == "" {
		return "", "", fmt.Errorf("could not find platform information from /etc/lsb-release: %v", lines)
	}

	platform = strings.ToLower(platform)
	return platform, platformVersion, nil
}

///////////////////////////
// /etc/${distrib}-release
//
// man 1 lsb_release
//
//   The "/etc/[distrib]-release" file contains a description line which is
//   parsed to get information (especially on currently non-LSB compliant
//   systems).
//
//   The required line style is:
//   "Distributor release x.x (Codename)"
///////////////////////////

func scanRhelFile(file string) (string, string, error) {
	var platform, platformVersion string
	var err error

	data, err := utils.ReadFileTrim(file)
	if err != nil {
		return "", "", err
	}

	platform, err = getRedhatishPlatform(data)
	if err != nil {
		return "", "", err
	}

	platformVersion, err = getRedhatishVersion(data)
	if err != nil {
		return "", "", err
	}

	return platform, platformVersion, nil
}

func getRedhatishPlatform(data string) (string, error) {
	mapping := []struct {
		regex    string
		platform string
	}{
		{`(?i)Red Hat Enterprise Linux`, c.PlatformRedhat},
		{`(?i)CentOS( Linux)?`, c.PlatformCentos},
		{`(?i)Fedora( Linux)?`, c.PlatformFedora},
		{`(?i)Amazon Linux`, c.PlatformAmazon},
	}

	for _, m := range mapping {
		if regexp.MustCompile(m.regex).MatchString(data) {
			return m.platform, nil
		}
	}

	return "", fmt.Errorf("could not detect RedHat platform: %s", string(data))
}

func getRedhatishVersion(data string) (string, error) {
	versionRexp := regexp.MustCompile(`^.*release ([0-9.]+).*$`)
	match := versionRexp.FindStringSubmatch(data)
	if len(match) > 0 {
		return string(match[1]), nil
	}
	return "", fmt.Errorf("could not detect RedHat Version: %s", string(data))
}

///////////////////////////
// distribution specifc /etc/${distri}-release files
// https://github.com/chef/ohai/blob/master/lib/ohai/plugins/linux/platform.rb#L131-L248
///////////////////////////

func scanDistributionReleaseFiles() (string, string, error) {
	var platform, platformVersion string
	var err error

	if _, err = os.Stat("/etc/debian_version"); err == nil {
		platform = c.PlatformDebian
		platformVersion, err = utils.ReadFileTrim("/etc/debian_version")
	} else if _, err = os.Stat("/etc/redhat-release"); err == nil {
		platform, platformVersion, err = scanRhelFile("/etc/redhat-release")
	} else if _, err = os.Stat("/etc/system-release"); err == nil {
		platform, platformVersion, err = scanRhelFile("/etc/system-release")
	} else if _, err = os.Stat("/etc/alpine-release"); err == nil {
		platform = c.PlatformAlpine
		platformVersion, err = utils.ReadFileTrim("/etc/alpine-release")
	} else {
		return "", "", errors.New("could not detect Linux platform")
	}

	return platform, platformVersion, err
}

///////////////////////////
// map platform to platform family
// https://github.com/chef/ohai/blob/master/lib/ohai/plugins/linux/platform.rb#L106-L129
///////////////////////////

func platformFamilyForPlatform(platform string) (string, error) {
	switch platform {
	case c.PlatformUbuntu, c.PlatformDebian, c.PlatformRaspbian:
		return c.PlatformFamilyDebian, nil
	case c.PlatformRedhat, c.PlatformCentos, c.PlatformAmazon:
		return c.PlatformFamilyRhel, nil
	case c.PlatformFedora:
		return c.PlatformFamilyFedora, nil
	case c.PlatformAlpine:
		return c.PlatformFamilyAlpine, nil
	case c.PlatformSuse, c.PlatformOpensuse, c.PlatformOpensuseLeap:
		return c.PlatformFamilySuse, nil
	case c.PlatformGentoo:
		return c.PlatformFamilyGentoo, nil
	case c.PlatformArch:
		return c.PlatformFamilyArch, nil
	default:
		return "", fmt.Errorf("unknown platform: %s", platform)
	}
}
