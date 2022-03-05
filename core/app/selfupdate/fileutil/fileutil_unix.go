// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package fileutil contains utilities for working with the file system.
package fileutil

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// Uncompress untar the installation package
func (futl *Fileutil) Uncompress(log log.T, src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	os.MkdirAll(dest, appconfig.ReadWriteExecuteAccess)

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		itemPath := dest + string(os.PathSeparator) + hdr.Name
		if !futl.isUnderDir(itemPath, dest) {
			return fmt.Errorf("%v attepts to place files outside %v subtree", file.Name(), dest)
		}
		if hdr.FileInfo().IsDir() {
			os.MkdirAll(itemPath, hdr.FileInfo().Mode())
		} else {
			mode := hdr.FileInfo().Mode()
			log.Debugf("Uncompressing file %v with %v mode", itemPath, mode.Perm().String())
			fw, err := os.OpenFile(itemPath, appconfig.FileFlagsCreateOrTruncate, mode)
			if err != nil {
				return err
			}
			defer fw.Close()

			_, err = io.Copy(fw, tr)
			if err != nil {
				return err
			}

			if err = os.Chmod(itemPath, mode); err != nil {
				return err
			}
			log.Debugf("Uncompressed file mode is %v", futl.GetFileMode(itemPath).Perm().String())
		}
	}
	return nil
}
