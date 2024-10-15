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

// Package artifact contains utilities for working downloading files.
package artifact

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/app/selfupdate/fileutil"
)

// DownloadOutput holds the result of file download operation.
type DownloadOutput struct {
	LocalFilePath string
	IsUpdated     bool
	IsHashMatched bool
}

// DownloadInput specifies the input to file download operation
type DownloadInput struct {
	SourceURL            string
	DestinationDirectory string
	SourceChecksums      map[string]string
}

type Artifact struct {
	log       log.T
	appConfig appconfig.SsmagentConfig
	fileutil  *fileutil.Fileutil
}

type IArtifact interface {
	Download(input DownloadInput) (output DownloadOutput, err error)
	VerifyHash(input DownloadInput, output DownloadOutput) (bool, error)
	Uncompress(src, dest string) error
}

func NewSelfUpdateArtifact(log log.T, appConfig appconfig.SsmagentConfig) *Artifact {
	log.Debugf("Initializing self update artifact")
	futl := fileutil.NewFileUtil(log)
	return &Artifact{
		log:       log,
		appConfig: appConfig,
		fileutil:  futl,
	}
}

// Download is a generic utility which attempts to download smartly.
func (artifact *Artifact) Download(input DownloadInput) (output DownloadOutput, err error) {
	// parse the url
	var fileURL *url.URL
	fileURL, err = url.Parse(input.SourceURL)
	if err != nil {
		err = fmt.Errorf("url parsing failed. %v", err)
		return
	}

	// create destination directory
	var destinationDir = input.DestinationDirectory
	if destinationDir == "" {
		destinationDir = appconfig.DownloadRoot
	}

	// create directory where artifacts are downloaded.
	err = artifact.fileutil.MakeDirs(destinationDir)
	if err != nil {
		err = fmt.Errorf("failed to create directory=%v, err=%v", destinationDir, err)
		return
	}

	// process if the url is local file or it has already been downloaded.
	var isLocalFile = false
	isLocalFile, err = artifact.fileutil.LocalFileExist(input.SourceURL)
	if err != nil {
		err = fmt.Errorf("check for local file exists returned %v", err)
		err = nil
	}

	if isLocalFile == true {
		// if local file exist, remove the downloaded artifacts and re-download it again.
		artifact.log.Debugf("source is a local file, start removing existing artifacts. %v", input.SourceURL)
		if err := artifact.fileutil.DeleteFile(input.SourceURL); err != nil {
			err = fmt.Errorf("source is a local file, failed to remove existing local file %v", input.SourceURL)
		} else {
			output.IsUpdated = false
		}
	}
	artifact.log.Debugf("attempt to get the source file as web download. %v", input.SourceURL)
	// compute the local filename which is hash of url_filename
	// Generating a hash_filename will also help against attackers
	// from specifying a directory and filename to overwrite any ami/built-in files.
	urlHash := sha1.Sum([]byte(fileURL.String()))
	output.LocalFilePath = filepath.Join(destinationDir, fmt.Sprintf("%x", urlHash))

	var tempOutput DownloadOutput

	artifact.log.Debugf("Try to download from http/https")
	tempOutput, err = artifact.httpDownload(input.SourceURL, output.LocalFilePath)
	output = tempOutput

	if err != nil {
		return
	}

	isLocalFile, err = artifact.fileutil.LocalFileExist(output.LocalFilePath)
	if isLocalFile == true {
		output.IsHashMatched, err = artifact.VerifyHash(input, output)
	}

	return
}

// httpDownload attempts to download a file via http/s call
func (artifact *Artifact) httpDownload(fileURL string, destFile string) (output DownloadOutput, err error) {
	artifact.log.Debugf("attempting to download as http/https download %v", destFile)
	eTagFile := destFile + ".etag"
	var check http.Client
	var request *http.Request
	request, err = http.NewRequest("GET", fileURL, nil)
	if err != nil {
		artifact.log.Errorf("Failed to create http request for artifact download %s", err)
		return
	}
	if artifact.fileutil.Exists(destFile) == true && artifact.fileutil.Exists(eTagFile) == true {
		var existingETag string
		existingETag, err = artifact.fileutil.ReadAllText(eTagFile)
		if err != nil {
			artifact.log.Errorf("Fail to read contents from exist file", err)
		}
		request.Header.Add("If-None-Match", existingETag)
	}

	check = http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	var resp *http.Response
	resp, err = check.Do(request)
	if err != nil {
		artifact.log.Debug("failed to download from http/https, ", err)
		artifact.fileutil.DeleteFile(destFile)
		artifact.fileutil.DeleteFile(eTagFile)
		return
	}

	if resp.StatusCode == http.StatusNotModified {
		artifact.log.Debug("Unchanged file.")
		output.IsUpdated = false
		output.LocalFilePath = destFile
		return output, nil
	} else if resp.StatusCode != http.StatusOK {
		artifact.log.Debug("failed to download from http/https, ", err)
		artifact.fileutil.DeleteFile(destFile)
		artifact.fileutil.DeleteFile(eTagFile)
		err = fmt.Errorf("http request failed. status:%v statuscode:%v", resp.Status, resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	eTagValue := resp.Header.Get("Etag")
	if eTagValue != "" {
		artifact.log.Debug("file eTagValue is ", eTagValue)
		err = artifact.fileutil.WriteAllText(eTagFile, eTagValue)
		if err != nil {
			artifact.log.Errorf("failed to write eTagfile %v, %v ", eTagFile, err)
			return
		}
	}
	_, err = artifact.fileCopy(destFile, resp.Body)
	if err == nil {
		output.LocalFilePath = destFile
		output.IsUpdated = true
	} else {
		artifact.log.Errorf("failed to write destFile %v, %v ", destFile, err)
	}
	return
}

// FileCopy copies the content from reader to destinationPath file
func (artifact *Artifact) fileCopy(destinationPath string, src io.Reader) (written int64, err error) {

	var file *os.File
	file, err = os.Create(destinationPath)
	if err != nil {
		artifact.log.Errorf("failed to create file. %v", err)
		return
	}
	defer file.Close()
	var size int64
	size, err = io.Copy(file, src)
	artifact.log.Debugf("%s with %v bytes downloaded", destinationPath, size)
	return
}

// VerifyHash verifies the hash of the url file as per specified hash algorithm type and its value
func (artifact *Artifact) VerifyHash(input DownloadInput, output DownloadOutput) (bool, error) {
	hasMatchingHash := false

	// check and set default hashing algorithm
	checksums := input.SourceChecksums

	if len(checksums) == 0 {
		return true, nil
	}

	//backwards compatibility for empty HashValues and HashTypes
	if len(checksums) == 1 {
		for _, hashValue := range checksums {
			// this is the only pair in the map
			if hashValue == "" {
				return true, nil
			}
		}
	}

	for hashAlgorithm, hashValue := range checksums {
		var computedHashValue string
		var err error
		// check the sha256 algorithm by default
		if hashAlgorithm == "" || strings.EqualFold(hashAlgorithm, "sha256") {
			computedHashValue, err = artifact.sha256HashValue(output.LocalFilePath)
		} else if strings.EqualFold(hashAlgorithm, "md5") {
			computedHashValue, err = artifact.md5HashValue(output.LocalFilePath)
		} else {
			continue
		}

		if err != nil {
			return false, fmt.Errorf("the algorithm returned an error when trying to compute the checksum %v", input)
		}

		if !strings.EqualFold(hashValue, computedHashValue) {
			return false, fmt.Errorf("failed to verify hash of downloadinput %v", input)
		}

		hasMatchingHash = true
	}

	//if a supported hash algorithm was not provided, jut return an error
	if !hasMatchingHash {
		return false, fmt.Errorf("no supported algorithm was provided for downloadinput %v", input)
	}

	return true, nil
}

// Sha256HashValue gets the sha256 hash value
func (artifact *Artifact) sha256HashValue(filePath string) (hash string, err error) {
	var exists = false
	exists, err = artifact.fileutil.LocalFileExist(filePath)
	if err != nil || exists == false {
		return
	}

	var f *os.File
	f, err = os.Open(filePath)
	if err != nil {
		artifact.log.Error(err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err = io.Copy(hasher, f); err != nil {
		artifact.log.Error(err)
	}
	hash = hex.EncodeToString(hasher.Sum(nil))
	artifact.log.Debugf("Hash=%v, FilePath=%v", hash, filePath)
	return
}

// Md5HashValue gets the md5 hash value
func (artifact *Artifact) md5HashValue(filePath string) (hash string, err error) {
	var exists = false
	exists, err = artifact.fileutil.LocalFileExist(filePath)
	if err != nil || exists == false {
		return
	}

	var f *os.File
	f, err = os.Open(filePath)
	if err != nil {
		artifact.log.Error(err)
	}
	defer f.Close()
	hasher := md5.New()
	if _, err = io.Copy(hasher, f); err != nil {
		artifact.log.Error(err)
	}
	hash = hex.EncodeToString(hasher.Sum(nil))
	artifact.log.Debugf("Hash=%v, FilePath=%v", hash, filePath)
	return
}

func (artifact *Artifact) Uncompress(src, dest string) error {
	return artifact.fileutil.Uncompress(artifact.log, src, dest)
}
