// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"fmt"
	"regexp"
	"strings"
)

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
