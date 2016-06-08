// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package artifact contains utilities for working downloading files.
package artifact

import (
	"crypto/md5"
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
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
	SourceHashValue      string
	SourceHashType       string
}

// httpDownload attempts to download a file via http/s call
func httpDownload(log log.T, fileURL string, destFile string) (output DownloadOutput, err error) {
	log.Debugf("attempting to download as http/https download %v", destFile)
	eTagFile := destFile + ".etag"
	var check http.Client
	var request *http.Request
	request, err = http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return
	}
	if fileutil.Exists(destFile) == true && fileutil.Exists(eTagFile) == true {
		var existingETag string
		existingETag, err = fileutil.ReadAllText(eTagFile)
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
		log.Debug("failed to download from http/https, ", err)
		fileutil.DeleteFile(destFile)
		fileutil.DeleteFile(eTagFile)
		return
	}

	if resp.StatusCode == http.StatusNotModified {
		log.Debugf("Unchanged file.")
		output.IsUpdated = false
		output.LocalFilePath = destFile
		return output, nil
	} else if resp.StatusCode != http.StatusOK {
		log.Debug("failed to download from http/https, ", err)
		fileutil.DeleteFile(destFile)
		fileutil.DeleteFile(eTagFile)
		err = fmt.Errorf("http request failed. status:%v statuscode:%v", resp.Status, resp.StatusCode)
		return
	}
	defer resp.Body.Close()
	eTagValue := resp.Header.Get("Etag")
	if eTagValue != "" {
		log.Debug("file eTagValue is ", eTagValue)
		err = fileutil.WriteAllText(eTagFile, eTagValue)
		if err != nil {
			log.Errorf("failed to write eTagfile %v, %v ", eTagFile, err)
			return
		}
	}
	_, err = FileCopy(log, destFile, resp.Body)
	if err == nil {
		output.LocalFilePath = destFile
		output.IsUpdated = true
	} else {
		log.Errorf("failed to write destFile %v, %v ", destFile, err)
	}
	return
}

// s3Download attempts to download a file via the aws sdk.
func s3Download(log log.T, amazonS3URL s3util.AmazonS3URL, destFile string) (output DownloadOutput, err error) {
	log.Debugf("attempting to download as s3 download %v", destFile)
	eTagFile := destFile + ".etag"

	config := &aws.Config{}
	var appConfig appconfig.SsmagentConfig
	appConfig, err = appconfig.Config(false)
	if err != nil {
		log.Error("failed to read appconfig.")
	} else {
		creds, err1 := appConfig.ProfileCredentials()
		if err1 != nil {
			config.Credentials = creds
		}
	}
	config.S3ForcePathStyle = aws.Bool(amazonS3URL.IsPathStyle)
	config.Region = aws.String(amazonS3URL.Region)

	params := &s3.GetObjectInput{
		Bucket: aws.String(amazonS3URL.Bucket),
		Key:    aws.String(amazonS3URL.Key),
	}

	if fileutil.Exists(destFile) == true && fileutil.Exists(eTagFile) == true {
		var existingETag string
		existingETag, err = fileutil.ReadAllText(eTagFile)
		if err != nil {
			log.Debugf("failed to read etag file %v, %v", eTagFile, err)
			return
		}
		params.IfNoneMatch = aws.String(existingETag)
	}

	s3client := s3.New(session.New(config))

	req, resp := s3client.GetObjectRequest(params)
	err = req.Send()
	if err != nil {
		if req.HTTPResponse == nil || req.HTTPResponse.StatusCode != http.StatusNotModified {
			log.Debug("failed to download from s3, ", err)
			fileutil.DeleteFile(destFile)
			fileutil.DeleteFile(eTagFile)
			return
		}

		log.Debugf("Unchanged file.")
		output.IsUpdated = false
		output.LocalFilePath = destFile
		return output, nil
	}

	if *resp.ETag != "" {
		log.Debug("files etag is ", *resp.ETag)
		err = fileutil.WriteAllText(eTagFile, *resp.ETag)
		if err != nil {
			log.Errorf("failed to write eTagfile %v, %v ", eTagFile, err)
			return
		}
	}

	defer resp.Body.Close()
	_, err = FileCopy(log, destFile, resp.Body)
	if err == nil {
		output.LocalFilePath = destFile
		output.IsUpdated = true
	} else {
		log.Errorf("failed to write destFile %v, %v ", destFile, err)
	}
	return
}

// FileCopy copies the content from reader to destinationPath file
func FileCopy(log log.T, destinationPath string, src io.Reader) (written int64, err error) {

	var file *os.File
	file, err = os.Create(destinationPath)
	if err != nil {
		log.Errorf("failed to create file. %v", err)
		return
	}
	defer file.Close()
	var size int64
	size, err = io.Copy(file, src)
	log.Infof("%s with %v bytes downloaded", destinationPath, size)
	return
}

// Download is a generic utility which attempts to download smartly.
func Download(log log.T, input DownloadInput) (output DownloadOutput, err error) {
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
	err = fileutil.MakeDirs(destinationDir)
	if err != nil {
		err = fmt.Errorf("failed to create directory=%v, err=%v", destinationDir, err)
		return
	}

	// process if the url is local file or it has already been downloaded.
	var isLocalFile = false
	isLocalFile, err = fileutil.LocalFileExist(input.SourceURL)
	if err != nil {
		err = fmt.Errorf("check for local file exists returned %v", err)
		err = nil
	}

	if isLocalFile == true {
		err = fmt.Errorf("source is a local file, skipping download. %v", input.SourceURL)
		output.LocalFilePath = input.SourceURL
		output.IsUpdated = false
		output.IsHashMatched, err = VerifyHash(log, input, output)
	} else {
		err = fmt.Errorf("source file wasn't found locally, will attempt as web download. %v", input.SourceURL)
		// compute the local filename which is hash of url_filename
		// Generating a hash_filename will also help against attackers
		// from specifying a directory and filename to overwrite any ami/built-in files.
		urlHash := md5.Sum([]byte(fileURL.String()))
		fileName := filepath.Base(fileURL.String())

		output.LocalFilePath = filepath.Join(destinationDir, fmt.Sprintf("%x_%v", urlHash, fileName))

		amazonS3URL := s3util.ParseAmazonS3URL(log, fileURL)
		if amazonS3URL.IsBucketAndKeyPresent() {
			// source is s3
			output, err = s3Download(log, amazonS3URL, output.LocalFilePath)
		} else {
			// simple httphttps download
			output, err = httpDownload(log, input.SourceURL, output.LocalFilePath)
		}
		if err != nil {
			return
		}

		isLocalFile, err = fileutil.LocalFileExist(output.LocalFilePath)
		if isLocalFile == true {
			output.IsHashMatched, err = VerifyHash(log, input, output)
		}
	}

	return
}

// VerifyHash verifies the hash of the url file as per specified hash algorithm type and its value
func VerifyHash(log log.T, input DownloadInput, output DownloadOutput) (match bool, err error) {
	match = false
	// if there is no hash specified, do not check
	if input.SourceHashValue == "" {
		match = true
		return
	}

	// check and set default hashing algorithm
	hashAlgorithm := input.SourceHashType
	if hashAlgorithm == "" {
		hashAlgorithm = "sha256"
	}

	computedHashValue := ""
	if strings.EqualFold(hashAlgorithm, "sha256") {
		computedHashValue, err = Sha256HashValue(log, output.LocalFilePath)

	} else if strings.EqualFold(hashAlgorithm, "md5") {
		computedHashValue, err = Md5HashValue(log, output.LocalFilePath)
	}
	if err != nil {
		return
	}
	match = strings.EqualFold(input.SourceHashValue, computedHashValue)
	if match == false {
		err = fmt.Errorf("failed to verify hash of downloadinput %v", input)
	}
	return
}

// Sha256HashValue gets the sha256 hash value
func Sha256HashValue(log log.T, filePath string) (hash string, err error) {
	var exists = false
	exists, err = fileutil.LocalFileExist(filePath)
	if err != nil || exists == false {
		return
	}

	var f *os.File
	f, err = os.Open(filePath)
	if err != nil {
		log.Error(err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err = io.Copy(hasher, f); err != nil {
		log.Error(err)
	}
	hash = hex.EncodeToString(hasher.Sum(nil))
	log.Debugf("Hash=%v, FilePath=%v", hash, filePath)
	return
}

// Md5HashValue gets the md5 hash value
func Md5HashValue(log log.T, filePath string) (hash string, err error) {
	var exists = false
	exists, err = fileutil.LocalFileExist(filePath)
	if err != nil || exists == false {
		return
	}

	var f *os.File
	f, err = os.Open(filePath)
	if err != nil {
		log.Error(err)
	}
	defer f.Close()
	hasher := md5.New()
	if _, err = io.Copy(hasher, f); err != nil {
		log.Error(err)
	}
	hash = hex.EncodeToString(hasher.Sum(nil))
	log.Debugf("Hash=%v, FilePath=%v", hash, filePath)
	return
}
