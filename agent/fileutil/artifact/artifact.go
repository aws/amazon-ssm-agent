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
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
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
	SourceChecksums      map[string]string
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

// awsConfig creates a config and sets region and credential information given an S3 URL
func awsConfig(log log.T, amazonS3URL s3util.AmazonS3URL) (config *aws.Config, err error) {
	config = sdkutil.AwsConfig()
	var appConfig appconfig.SsmagentConfig
	appConfig, errConfig := appconfig.Config(false)
	if errConfig != nil {
		log.Error("failed to read appconfig.")
	} else {
		if appConfig.S3.Endpoint != "" {
			config.Endpoint = &appConfig.S3.Endpoint
		} else {
			if region, err := platform.Region(); err == nil {
				if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "s3"); defaultEndpoint != "" {
					config.Endpoint = &defaultEndpoint
				}
			} else {
				log.Errorf("error fetching the region, %v", err)
			}
		}
	}
	config.S3ForcePathStyle = aws.Bool(amazonS3URL.IsPathStyle)
	config.Region = aws.String(amazonS3URL.Region)
	return config, nil
}

// CanGetS3Object returns true if it is possible to fetch an object because it exists, is not deleted, and read permissions exist for this request
func CanGetS3Object(log log.T, amazonS3URL s3util.AmazonS3URL) bool {
	config, _ := awsConfig(log, amazonS3URL)
	bucketName := amazonS3URL.Bucket
	objectKey := amazonS3URL.Key

	params := &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}

	appConfig, _ := appconfig.Config(false)
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	s3client := s3.New(sess)
	var res *s3.HeadObjectOutput
	var err error
	if res, err = s3client.HeadObject(params); err != nil {
		log.Debugf("CanGetS3Object err: %v", err)
		return false
	}
	// Even with versioning on, a deleted object should return a 404, but to be certain, exclude delete markers explicitly
	return res.DeleteMarker == nil || !*(res.DeleteMarker)
}

// ListS3Folders returns the folders under a given S3 URL where folders are keys whose prefix is the URL key
// and contain a / after the prefix.  The folder name is the part between the prefix and the /.
func ListS3Folders(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	config, _ := awsConfig(log, amazonS3URL)
	prefix := amazonS3URL.Key
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	params := &s3.ListObjectsInput{
		Bucket:    aws.String(amazonS3URL.Bucket),
		Prefix:    &prefix,
		Delimiter: aws.String("/"),
	}
	appConfig, _ := appconfig.Config(false)
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	s3client := s3.New(sess)
	req, resp := s3client.ListObjectsRequest(params)
	err = req.Send()
	log.Debugf("ListS3Folders Bucket: %v, Prefix: %v, RequestID: %v", params.Bucket, params.Prefix, req.RequestID)
	if err != nil {
		log.Debugf("ListS3Folders error %v", err.Error())
		return
	}
	//TODO:MF: This works, but the string trimming required makes me think there should be some easier way to get this information
	//TODO:MF: Check IsTruncated and if so, make additional request(s) with Marker - currently we're limited to 1000 results
	folders := make([]string, 0)
	for _, key := range resp.CommonPrefixes {
		folders = append(folders, strings.TrimRight(strings.Replace(*key.Prefix, prefix, "", -1), "/"))
	}
	return folders, nil
}

// ListS3Directory returns all the objects (files and folders) under a given S3 URL where folders are keys whose prefix
// is the URL key and contain a / after the prefix.
func ListS3Directory(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	config, _ := awsConfig(log, amazonS3URL)
	var params *s3.ListObjectsInput
	prefix := amazonS3URL.Key
	if prefix != "" {
		// appending "/" if it does not already exist
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		params = &s3.ListObjectsInput{
			Bucket: aws.String(amazonS3URL.Bucket),
			Prefix: &prefix,
		}
	} else {
		params = &s3.ListObjectsInput{
			Bucket: aws.String(amazonS3URL.Bucket),
		}
	}
	log.Debugf("ListS3Object Bucket: %v, Prefix: %v", params.Bucket, params.Prefix)

	appConfig, _ := appconfig.Config(false)
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	s3client := s3.New(sess)
	obj, err := s3client.ListObjects(params)
	if err != nil {
		log.Errorf("ListS3Directory error %v", err.Error())
		return folderNames, err
	}

	log.Debugf("Contents %v ", obj.Contents)
	for i, contents := range obj.Contents {
		folderNames = append(folderNames, *contents.Key)
		log.Debug("Name of file/folder - ", folderNames[i])
	}
	return
}

// s3Download attempts to download a file via the aws sdk.
func s3Download(log log.T, amazonS3URL s3util.AmazonS3URL, destFile string) (output DownloadOutput, err error) {
	log.Debugf("attempting to download as s3 download %v", destFile)
	eTagFile := destFile + ".etag"

	config, _ := awsConfig(log, amazonS3URL)
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
	appConfig, _ := appconfig.Config(false)
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	s3client := s3.New(sess)

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
		urlHash := sha1.Sum([]byte(fileURL.String()))
		output.LocalFilePath = filepath.Join(destinationDir, fmt.Sprintf("%x", urlHash))

		amazonS3URL := s3util.ParseAmazonS3URL(log, fileURL)
		if amazonS3URL.IsBucketAndKeyPresent() {
			// source is s3
			var tempOutput DownloadOutput
			tempOutput, err = s3Download(log, amazonS3URL, output.LocalFilePath)
			// if s3 download fails, attempt http/https download as fallback
			if err != nil {
				tempOutput, err = httpDownload(log, input.SourceURL, output.LocalFilePath)
			}
			output = tempOutput
		} else {
			// simple http/https download
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
func VerifyHash(log log.T, input DownloadInput, output DownloadOutput) (bool, error) {
	hasMatchingHash := false

	// check and set default hashing algorithm
	checksums := input.SourceChecksums

	if len(checksums) == 0 {
		return true, nil
	}

	//backwards compatibility for empty HashValues and HashTypes
	if len(checksums) == 1 {
		for hashAlgorithm, hashValue := range checksums {
			// this is the only pair in the map
			if hashAlgorithm == "" || hashValue == "" {
				return true, nil
			}
		}
	}

	for hashAlgorithm, hashValue := range checksums {
		var computedHashValue string
		var err error
		// check the sha256 algorithm by default
		if hashAlgorithm == "" || strings.EqualFold(hashAlgorithm, "sha256") {
			computedHashValue, err = Sha256HashValue(log, output.LocalFilePath)
		} else if strings.EqualFold(hashAlgorithm, "md5") {
			computedHashValue, err = Md5HashValue(log, output.LocalFilePath)
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
