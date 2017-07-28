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

// +build darwin freebsd linux netbsd openbsd

// Package file contains file gatherer.
package file

import (
	"os"

	"path/filepath"

	"strconv"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

func expand(str string, mapping func(string) string) (newStr string, err error) {
	newStr = os.Expand(str, mapping)
	return
}

//getMetaData gets metadata for the specified file paths
func getMetaData(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			LogError(log, err)
		} else {
			var data model.FileData
			data.Size = strconv.FormatInt(fi.Size(), 10)
			data.Name = fi.Name()
			data.ModificationTime = fi.ModTime().Format(time.RFC3339)
			data.InstalledDir = filepath.Dir(p)
			fileInfo = append(fileInfo, data)
		}
	}
	return
}
