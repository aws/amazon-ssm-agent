package versionutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"
)

// ByVersion implements sorting versions (by component) using semver if applicable
type ByVersion []string

func (s ByVersion) Len() int {
	return len(s)
}

func (s ByVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByVersion) Less(i, j int) bool {
	//we need to compare string by ignoring it's case
	return Compare(s[i], s[j], true) < 0
}

// Compare returns 0 if two versions are equal a negative number if this < other and a positive number if this > other
// If this and other are both compliant with semver, then semver sorting rules are used
// Otherwise the versions are compared component-by-component, numerically if both are numeric
// If !strictSort insignificant trailing components are ignored (1.0.0.0 == 1) and the alpha comparison is case-insensitive
func Compare(this string, other string, strictSort bool) int {
	// If both versions are compliant with SemVer, use the SemVer comparison rules
	thisSemVer, thisSemErr := semver.NewVersion(this)
	otherSemVer, otherSemErr := semver.NewVersion(other)
	if thisSemErr == nil && otherSemErr == nil {
		return thisSemVer.Compare(*otherSemVer)
	}

	thisVersion := this
	otherVersion := other

	if !strictSort {
		// Unless we need a strict ordering, trailing 0 components of version should be ignored
		thisVersion = normalizeForCompare(thisVersion)
		otherVersion = normalizeForCompare(otherVersion)
	}

	thisComponents := strings.Split(thisVersion, ".")
	otherComponents := strings.Split(otherVersion, ".")

	for i := 0; i < len(thisComponents) && i < len(otherComponents); i++ {
		thisNum, errThis := strconv.Atoi(thisComponents[i])
		otherNum, errOther := strconv.Atoi(otherComponents[i])
		isNumeric := errThis == nil && errOther == nil
		if isNumeric {
			// If we can compare numbers, compare numbers
			if thisNum < otherNum {
				return -1
			} else if thisNum > otherNum {
				return 1
			}
		} else {
			// If either component is not numeric, compare them as text
			if thisComponents[i] < otherComponents[i] {
				return -1
			} else if thisComponents[i] > otherComponents[i] {
				return 1
			}
		}
	}
	// If all components are equal until we run out of components then the
	// version with fewer components is the lesser version
	return len(thisComponents) - len(otherComponents)
}

// normalizeForCompare removes components at the end that are numerically equal to 0
func normalizeForCompare(version string) string {
	if len(version) == 0 {
		return version
	}
	lenSignificant := len(version)
	for i := len(version) - 1; i >= 0; i-- {
		if version[i] != '0' && version[i] != '.' {
			break
		}
		if version[i] == '.' {
			lenSignificant = i
		}
		if i == 0 {
			lenSignificant = 0
		}
	}
	return version[0:lenSignificant]
}

func IsValidVersion(version string) bool {
	_, err := versionOrdinal(strings.TrimSpace(version))
	return err == nil
}

// VersionCompare compares two version strings
func VersionCompare(versionl string, versionr string) (result int, err error) {
	if versionl, err = versionOrdinal(strings.TrimSpace(versionl)); err != nil {
		return 0, err
	}
	if versionr, err = versionOrdinal(strings.TrimSpace(versionr)); err != nil {
		return 0, err
	}

	if versionl < versionr {
		return -1, nil
	} else if versionl > versionr {
		return 1, nil
	} else {
		return 0, nil
	}
}

func versionOrdinal(version string) (string, error) {
	// validate if string is a valid version string
	if matched, err := regexp.MatchString("\\d+(\\.\\d+)?", version); matched == false || err != nil {
		return "", fmt.Errorf("Invalid version string %v", version)
	}

	// ISO/IEC 14651:2011
	const maxByte = 1<<8 - 1
	vo := make([]byte, 0, len(version)+8)
	j := -1
	for i := 0; i < len(version); i++ {
		b := version[i]
		if '0' > b || b > '9' {
			vo = append(vo, b)
			j = -1
			continue
		}
		if j == -1 {
			vo = append(vo, 0x00)
			j = len(vo) - 1
		}
		if vo[j] == 1 && vo[j+1] == '0' {
			vo[j+1] = b
			continue
		}
		if vo[j]+1 > maxByte {
			return "", fmt.Errorf("VersionOrdinal: invalid version")
		}
		vo = append(vo, b)
		vo[j]++
	}
	return string(vo), nil
}
