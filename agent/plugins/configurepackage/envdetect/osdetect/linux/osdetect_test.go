package linux

import (
	"fmt"
	"testing"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestParseLSBreleaseFile(t *testing.T) {
	data := []struct {
		input            []string
		expectedPlatform string
		expectedVersion  string
		expectError      bool
	}{
		{
			[]string{"DISTRIB_ID=Ubuntu", "DISTRIB_RELEASE=12.04", "DISTRIB_CODENAME=precise", "DISTRIB_DESCRIPTION=\"Ubuntu 12.04.5 LTS\""},
			"ubuntu", "12.04", false,
		},
		{
			[]string{"DISTRIB_ID=Ubuntu\n"},
			"ubuntu", "", false,
		},
		{
			[]string{"DISTRIB_RELEASE=12.04\n"},
			"", "12.04", false,
		},
		{
			[]string{"DISTRIB_ID=Ubuntu"},
			"ubuntu", "", false,
		},
		{
			[]string{"DISTRIB_ID='Ubuntu'"},
			"ubuntu", "", false,
		},
		{
			[]string{"DISTRIB_ID=\"Ubuntu\""},
			"ubuntu", "", false,
		},
		{
			[]string{"DISTRIB_ID=\"SUSE\"", "DISTRIB_RELEASE=12.2"},
			"suse", "12.2", false,
		},
		{
			[]string{""},
			"", "", true,
		},
		{
			[]string{},
			"", "", true,
		},
		{
			[]string{"LSB_VERSION=base-4.0-amd64:base-4.0-noarch:core-4.0-amd64:core-4.0-noarch:graphics-4.0-amd64:graphics-4.0-noarch:printing-4.0-amd64:printing-4.0-noarch"},
			"", "", true,
		},
	}

	for _, m := range data {
		t.Run(fmt.Sprintf("%s in (%s, %s)", m.input, m.expectedPlatform, m.expectedVersion), func(t *testing.T) {
			resultPlatform, resultVersion, err := parseLSBreleaseFile(m.input)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expectedPlatform, resultPlatform)
			assert.Equal(t, m.expectedVersion, resultVersion)
		})
	}
}

func TestParseLSBreleaseCMD(t *testing.T) {
	data := []struct {
		inputID          []byte
		inputRelease     []byte
		expectedPlatform string
		expectedVersion  string
	}{
		{
			[]byte(""), []byte(""),
			"", "",
		},
		{
			[]byte("ASdF"), []byte("J:Kö"),
			"asdf", "j:kö",
		},
		{
			[]byte("Debian\n"), []byte("6.7\n"),
			c.PlatformDebian, "6.7",
		},
		{
			[]byte("Ubuntu"), []byte("14.04"),
			c.PlatformUbuntu, "14.04",
		},
		{
			[]byte("RedHatEnterpriseServer"), []byte("6.7"),
			c.PlatformRedhat, "6.7",
		},
		{
			[]byte("SUSE LINUX"), []byte("42.1"),
			c.PlatformSuse, "42.1",
		},
		{
			[]byte("openSUSE project"), []byte("13.2"),
			c.PlatformOpensuse, "13.2",
		},
		{
			[]byte("Debian"), []byte("6.7"),
			c.PlatformDebian, "6.7",
		},
		{
			[]byte("Gentoo"), []byte("2.3"),
			c.PlatformGentoo, "2.3",
		},
	}

	for _, m := range data {
		t.Run(fmt.Sprintf("(%s, %s) in (%s, %s)", m.inputID, m.inputRelease, m.expectedPlatform, m.expectedVersion), func(t *testing.T) {
			resultPlatform, resultVersion := parseLSBreleaseCMD(m.inputID, m.inputRelease)
			assert.Equal(t, m.expectedPlatform, resultPlatform)
			assert.Equal(t, m.expectedVersion, resultVersion)
		})
	}
}

func TestParseOSreleaseFile(t *testing.T) {
	data := []struct {
		input            []string
		expectedPlatform string
		expectedVersion  string
		expectError      bool
	}{
		{
			[]string{`NAME="Ubuntu"`, `VERSION="12.04.5 LTS, Precise Pangolin"`, `ID=ubuntu`, `ID_LIKE=debian`, `PRETTY_NAME="Ubuntu precise (12.04.5 LTS)"`, `VERSION_ID="12.04"`},
			"ubuntu", "12.04", false,
		},
		{
			[]string{`NAME="Ubuntu"`, `VERSION="14.04.5 LTS, Trusty Tahr"`, `ID=ubuntu`, `ID_LIKE=debian`, `PRETTY_NAME="Ubuntu 14.04.5 LTS"`, `VERSION_ID="14.04"`, `HOME_URL="http://www.ubuntu.com/"`, `SUPPORT_URL="http://help.ubuntu.com/"`, `BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`},
			"ubuntu", "14.04", false,
		},
		{
			[]string{`NAME="Ubuntu"`, `VERSION="16.04.1 LTS (Xenial Xerus)"`, `ID=ubuntu`, `ID_LIKE=debian`, `PRETTY_NAME="Ubuntu 16.04.1 LTS"`, `VERSION_ID="16.04"`, `HOME_URL="http://www.ubuntu.com/"`, `SUPPORT_URL="http://help.ubuntu.com/"`, `BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`, `VERSION_CODENAME=xenial`, `UBUNTU_CODENAME=xenial`},
			"ubuntu", "16.04", false,
		},
		{
			[]string{`NAME="Ubuntu"`, `VERSION="17.04 (Zesty Zapus)"`, `ID=ubuntu`, `ID_LIKE=debian`, `PRETTY_NAME="Ubuntu Zesty Zapus (development branch)"`, `VERSION_ID="17.04"`, `HOME_URL="http://www.ubuntu.com/"`, `SUPPORT_URL="http://help.ubuntu.com/"`, `BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`, `PRIVACY_POLICY_URL="http://www.ubuntu.com/legal/terms-and-policies/privacy-policy"`, `VERSION_CODENAME=zesty`, `UBUNTU_CODENAME=zesty`},
			"ubuntu", "17.04", false,
		},
		{
			[]string{`PRETTY_NAME="Debian GNU/Linux 7 (wheezy)"`, `NAME="Debian GNU/Linux"`, `VERSION_ID="7"`, `VERSION="7 (wheezy)"`, `ID=debian`, `ANSI_COLOR="1;31"`, `HOME_URL="http://www.debian.org/"`, `SUPPORT_URL="http://www.debian.org/support/"`, `BUG_REPORT_URL="http://bugs.debian.org/"`},
			"debian", "7", false,
		},
		{
			[]string{`PRETTY_NAME="Debian GNU/Linux 8 (jessie)"`, `NAME="Debian GNU/Linux"`, `VERSION_ID="8"`, `VERSION="8 (jessie)"`, `ID=debian`, `HOME_URL="http://www.debian.org/"`, `SUPPORT_URL="http://www.debian.org/support"`, `BUG_REPORT_URL="https://bugs.debian.org/"`},
			"debian", "8", false,
		},
		{
			[]string{`NAME="Alpine Linux"`, `ID=alpine`, `VERSION_ID=3.1.4`, `PRETTY_NAME="Alpine Linux v3.1"`, `HOME_URL="http://alpinelinux.org"`, `BUG_REPORT_URL="http://bugs.alpinelinux.org"`},
			"alpine", "3.1.4", false,
		},
		{
			[]string{`NAME="Alpine Linux"`, `ID=alpine`, `VERSION_ID=3.5.0`, `PRETTY_NAME="Alpine Linux v3.5"`, `HOME_URL="http://alpinelinux.org"`, `BUG_REPORT_URL="http://bugs.alpinelinux.org"`},
			"alpine", "3.5.0", false,
		},
		{
			[]string{`NAME="CentOS Linux"`, `VERSION="7 (Core)"`, `ID="centos"`, `ID_LIKE="rhel fedora"`, `VERSION_ID="7"`, `PRETTY_NAME="CentOS Linux 7 (Core)"`, `ANSI_COLOR="0;31"`, `CPE_NAME="cpe:/o:centos:centos:7"`, `HOME_URL="https://www.centos.org/"`, `BUG_REPORT_URL="https://bugs.centos.org/"`, `CENTOS_MANTISBT_PROJECT="CentOS-7"`, `CENTOS_MANTISBT_PROJECT_VERSION="7"`, `REDHAT_SUPPORT_PRODUCT="centos"`, `REDHAT_SUPPORT_PRODUCT_VERSION="7"`},
			"centos", "7", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="20 (Heisenbug)"`, `ID=fedora`, `VERSION_ID=20`, `PRETTY_NAME="Fedora 20 (Heisenbug)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:20"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=20`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=20`},
			"fedora", "20", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="21 (Twenty One)"`, `ID=fedora`, `VERSION_ID=21`, `PRETTY_NAME="Fedora 21 (Twenty One)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:21"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=21`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=21`},
			"fedora", "21", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="22 (Twenty Two)"`, `ID=fedora`, `VERSION_ID=22`, `PRETTY_NAME="Fedora 22 (Twenty Two)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:22"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=22`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=22`, `PRIVACY_POLICY_URL=https://fedoraproject.org/wiki/Legal:PrivacyPolicy`},
			"fedora", "22", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="23 (Twenty Three)"`, `ID=fedora`, `VERSION_ID=23`, `PRETTY_NAME="Fedora 23 (Twenty Three)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:23"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=23`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=23`, `PRIVACY_POLICY_URL=https://fedoraproject.org/wiki/Legal:PrivacyPolicy`},
			"fedora", "23", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="24 (Twenty Four)"`, `ID=fedora`, `VERSION_ID=24`, `PRETTY_NAME="Fedora 24 (Twenty Four)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:24"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=24`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=24`, `PRIVACY_POLICY_URL=https://fedoraproject.org/wiki/Legal:PrivacyPolicy`},
			"fedora", "24", false,
		},
		{
			[]string{`NAME=Fedora`, `VERSION="25 (Twenty Five)"`, `ID=fedora`, `VERSION_ID=25`, `PRETTY_NAME="Fedora 25 (Twenty Five)"`, `ANSI_COLOR="0;34"`, `CPE_NAME="cpe:/o:fedoraproject:fedora:25"`, `HOME_URL="https://fedoraproject.org/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Fedora"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=25`, `REDHAT_SUPPORT_PRODUCT="Fedora"`, `REDHAT_SUPPORT_PRODUCT_VERSION=25`, `PRIVACY_POLICY_URL=https://fedoraproject.org/wiki/Legal:PrivacyPolicy`},
			"fedora", "25", false,
		},
		{
			[]string{`NAME="Red Hat Enterprise Linux Server"`, `VERSION="7.3 (Maipo)"`, `ID="rhel"`, `ID_LIKE="fedora"`, `VERSION_ID="7.3"`, `PRETTY_NAME="Red Hat Enterprise Linux Server 7.3 (Maipo)"`, `ANSI_COLOR="0;31"`, `CPE_NAME="cpe:/o:redhat:enterprise_linux:7.3:GA:server"`, `HOME_URL="https://www.redhat.com/"`, `BUG_REPORT_URL="https://bugzilla.redhat.com/"`, `REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 7"`, `REDHAT_BUGZILLA_PRODUCT_VERSION=7.3`, `REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"`, `REDHAT_SUPPORT_PRODUCT_VERSION="7.3"`},
			"redhat", "7.3", false,
		},
		{
			[]string{`NAME="Amazon Linux AMI"`, `VERSION="2016.09"`, `ID="amzn"`, `ID_LIKE="rhel fedora"`, `VERSION_ID="2016.09"`, `PRETTY_NAME="Amazon Linux AMI 2016.09"`, `ANSI_COLOR="0;33"`, `CPE_NAME="cpe:/o:amazon:linux:2016.09:ga"`, `HOME_URL="http://aws.amazon.com/amazon-linux-ami/"`},
			"amazon", "2016.09", false,
		},
		{
			[]string{`NAME="openSUSE Leap"`, `VERSION="42.1"`, `VERSION_ID="42.1"`, `PRETTY_NAME="openSUSE Leap 42.1 (x86_64)"`, `ID=opensuse`, `ANSI_COLOR="0;32"`, `CPE_NAME="cpe:/o:opensuse:opensuse:42.1"`, `BUG_REPORT_URL="https://bugs.opensuse.org"`, `HOME_URL="https://opensuse.org/"`, `ID_LIKE="suse"`},
			"opensuseleap", "42.1", false,
		},
		{
			[]string{`NAME="SLES"`, `VERSION="12-SP2"`, `VERSION_ID="12.2"`, `PRETTY_NAME="SUSE Linux Enterprise Server 12 SP2"`, `ID="sles"`, `ANSI_COLOR="0;32"`, `CPE_NAME="cpe:/o:suse:sles:12:sp2"`},
			"suse", "12.2", false,
		},
		{
			[]string{`NAME=openSUSE`, `VERSION="13.2 (Harlequin)"`, `VERSION_ID="13.2"`, `PRETTY_NAME="openSUSE 13.2 (Harlequin) (x86_64)"`, `ID=opensuse`, `ANSI_COLOR="0;32"`, `CPE_NAME="cpe:/o:opensuse:opensuse:13.2"`, `BUG_REPORT_URL="https://bugs.opensuse.org"`, `HOME_URL="https://opensuse.org/"`, `ID_LIKE="suse`},
			"opensuse", "13.2", false,
		},
		{
			[]string{`NAME=Gentoo`, `ID=gentoo`, `PRETTY_NAME="Gentoo/Linux"`, `ANSI_COLOR="1;32"`, `HOME_URL="https://www.gentoo.org/"`, `SUPPORT_URL="https://www.gentoo.org/support/"`, `BUG_REPORT_URL="https://bugs.gentoo.org/"`},
			"gentoo", "", false,
		},
		{
			[]string{`VERSION_ID="12.2"`, `ID="suse"`},
			"suse", "12.2", false,
		},
		{
			[]string{""},
			"", "", true,
		},
		{
			[]string{},
			"", "", true,
		},
	}
	for _, m := range data {
		t.Run(fmt.Sprintf("lsb-release for (%s, %s)", m.expectedPlatform, m.expectedVersion), func(t *testing.T) {
			resultPlatform, resultVersion, err := parseOSreleaseFile(m.input)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expectedPlatform, resultPlatform)
			assert.Equal(t, m.expectedVersion, resultVersion)
		})
	}
}

func TestGetRedhatishPlatform(t *testing.T) {
	data := []struct {
		content     string
		expected    string
		expectError bool
	}{
		{"CentOS release 5.11 (Final)", "centos", false},
		{"CentOS release 6.8 (Final)", "centos", false},
		{"CentOS Linux release 7.3.1611 (Core)", "centos", false},
		{"Fedora release 20 (Heisenbug)", "fedora", false},
		{"Fedora release 21 (Twenty One)", "fedora", false},
		{"Fedora release 22 (Twenty Two)", "fedora", false},
		{"Fedora release 23 (Twenty Three)", "fedora", false},
		{"Fedora release 24 (Twenty Four)", "fedora", false},
		{"Fedora release 25 (Twenty Five)", "fedora", false},
		{"Red Hat Enterprise Linux Server release 7.3 (Maipo)", "redhat", false},
		{"Red Hat Enterprise Linux Server release 6.8 (Santiago)", "redhat", false},
		{"Amazon Linux AMI release 2016.09", "amazon", false},
		{"asdf", "", true},
		{"", "", true},
	}
	for _, m := range data {
		t.Run(fmt.Sprintf("%s in %s", m.content, m.expected), func(t *testing.T) {
			result, err := getRedhatishPlatform(m.content)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expected, result)
		})
	}
}

func TestGetRedhatishVersion(t *testing.T) {
	data := []struct {
		content     string
		expected    string
		expectError bool
	}{
		{"CentOS release 5.11 (Final)", "5.11", false},
		{"CentOS release 6.8 (Final)", "6.8", false},
		{"CentOS Linux release 7.3.1611 (Core)", "7.3.1611", false},
		{"Fedora release 20 (Heisenbug)", "20", false},
		{"Fedora release 21 (Twenty One)", "21", false},
		{"Fedora release 22 (Twenty Two)", "22", false},
		{"Fedora release 23 (Twenty Three)", "23", false},
		{"Fedora release 24 (Twenty Four)", "24", false},
		{"Fedora release 25 (Twenty Five)", "25", false},
		{"Red Hat Enterprise Linux Server release 7.3 (Maipo)", "7.3", false},
		{"Red Hat Enterprise Linux Server release 6.8 (Santiago)", "6.8", false},
		{"Amazon Linux AMI release 2016.09", "2016.09", false},
		{"asdf", "", true},
		{"", "", true},
	}
	for _, m := range data {
		t.Run(fmt.Sprintf("%s in %s", m.content, m.expected), func(t *testing.T) {
			result, err := getRedhatishVersion(m.content)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expected, result)
		})
	}
}

// https://github.com/chef/ohai/blob/master/lib/ohai/plugins/linux/platform.rb#L106-L129
func TestPlatformFamilyForPlatform(t *testing.T) {
	data := []struct {
		platform    string
		expected    string
		expectError bool
	}{
		{"debian", "debian", false},
		{"ubuntu", "debian", false},
		{"centos", "rhel", false},
		{"redhat", "rhel", false},
		{"amazon", "rhel", false},
		{"suse", "suse", false},
		{"opensuse", "suse", false},
		{"opensuseleap", "suse", false},
		{"fedora", "fedora", false},
		{"gentoo", "gentoo", false},
		{"arch", "arch", false},
		{"alpine", "alpine", false},
		{"asdf", "", true},
	}

	for _, m := range data {
		t.Run(fmt.Sprintf("%s in %s", m.platform, m.expected), func(t *testing.T) {
			result, err := platformFamilyForPlatform(m.platform)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expected, result)
		})
	}
}

func TestDetectPackageManager(t *testing.T) {
	data := []struct {
		platform    string
		version     string
		family      string
		expected    string
		expectError bool
	}{
		{"", "", "", "", true},
		{"", "", "asdf", "", true},
		{"", "", "windows", "", true},
		{"", "", "mac_os_x", "", true},
		{"", "", "debian", "apt", false},
		{"", "", "rhel", "yum", false},
		{"", "", "fedora", "dnf", false},
		{"", "", "alpine", "alpine", false},
		{"", "", "suse", "zypper", false},
		{"", "", "gentoo", "emerge", false},
		{"", "", "arch", "pacman", false},
	}

	for _, m := range data {
		t.Run(fmt.Sprintf("(%s,%s,%s) in %s", m.platform, m.version, m.family, m.expected), func(t *testing.T) {
			d := Detector{}
			result, err := d.DetectPkgManager(m.platform, m.version, m.family)

			if m.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, m.expected, result)
		})
	}
}
