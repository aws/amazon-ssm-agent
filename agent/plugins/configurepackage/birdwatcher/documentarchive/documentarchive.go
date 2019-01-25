// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package documentarchive contains the struct that is called when the package information is stored in birdwatcher
package documentarchive

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"

	"github.com/aws/aws-sdk-go/service/ssm"
)

type PackageArchive struct {
	facadeClient   facade.BirdwatcherFacade
	manifestCache  packageservice.ManifestCache
	archiveType    string
	timeUnit       int
	localDocuments map[string]*localDocument
}

type localDocument struct {
	documentArn string
	docVersion  string
	docHash     string
	attachments []*ssm.AttachmentContent
}

//constructors
// New is a constructor for PackageArchive struct
func New(facadeClientSession facade.BirdwatcherFacade) archive.IPackageArchive {
	localDocuments := make(map[string]*localDocument)
	return &PackageArchive{
		facadeClient:   facadeClientSession,
		archiveType:    archive.PackageArchiveDocument,
		timeUnit:       1000, //Adding here to be able to change for testing, 1000 millisecond to make a second
		localDocuments: localDocuments,
	}
}

// NewDocumentArchive is a constructor for PackageArchive struct (meant for testing)
func NewDocumentArchive(facadeClientSession facade.BirdwatcherFacade, attachmentsContent []*ssm.AttachmentContent, docDescription *ssm.DocumentDescription, cache packageservice.ManifestCache, packageName string, version string, manifestStr string) archive.IPackageArchive {
	localDocuments := make(map[string]*localDocument)
	key := archive.FormKey(packageName, version)
	localDocuments[key] = &localDocument{
		documentArn: *docDescription.Name,
		docVersion:  *docDescription.DocumentVersion,
		docHash:     *docDescription.Hash,
		attachments: attachmentsContent,
	}

	return &PackageArchive{
		facadeClient:   facadeClientSession,
		archiveType:    archive.PackageArchiveDocument,
		manifestCache:  cache,
		timeUnit:       0, //No sleep for testing
		localDocuments: localDocuments,
	}
}

// Name of archive type
func (da *PackageArchive) Name() string {
	return da.archiveType
}

//setters
// SetPackageName sets the document arn. The manifest is not required
// since we use the document name and document version
func (da *PackageArchive) SetResource(packageName string, version string, manifest *birdwatcher.Manifest) {
	// No need to do anything since it's already set during the. TODO: Refactor this.
	return
}

// SetManifestCache sets the manifest Cache
func (da *PackageArchive) SetManifestCache(manifestCache packageservice.ManifestCache) {
	da.manifestCache = manifestCache
}

//getters
// GetResourceVersion makes a call to birdwatcher API to figure the right version of the resource that needs to be installed
func (da *PackageArchive) GetResourceVersion(packageName string, packageVersion string) (name string, version string) {
	// Return the packageVersion as "" if empty and return version if specified.
	return packageName, packageVersion
}

// GetResourceArn returns the document Arn required for storing the file. This is found in the response of GetDocument.
func (da *PackageArchive) GetResourceArn(packageName string, version string) string {
	// GetDocument returns the Name of the document if it belongs to the account of the instance.
	// If it is a shared document, GetDocument returns the document ARN as Name
	key := archive.FormKey(packageName, version)
	if _, ok := da.localDocuments[key]; !ok {
		return ""
	}

	return da.localDocuments[key].documentArn
}

// GetFileDownloadLocation obtains the location of the file in the archive
// in the document archive, this information is stored in the attachmentContent
// field in the reult of GetDocument.
func (da *PackageArchive) GetFileDownloadLocation(file *archive.File, packageName string, version string) (string, error) {
	if file == nil {
		return "", errors.New("Could not obtain the file from manifest")
	}
	fileName := file.Name

	key := archive.FormKey(packageName, version)
	_, ok := da.localDocuments[key]
	if !ok || da.localDocuments[key].attachments == nil {
		if _, err := da.getDocument(packageName, version); err != nil {
			return "", err
		}
	}

	// if the attachments are still nil, return error
	if da.localDocuments[key].attachments == nil {
		return "", fmt.Errorf("Could not retreive the attachments for installation")
	}

	for _, attachmentContent := range da.localDocuments[key].attachments {
		if *attachmentContent.Name == fileName {
			return *attachmentContent.Url, nil
		}
	}

	return "", fmt.Errorf("Install attachments for package does not exist")

}

// DownloadArtifactInfo downloads the document using GetDocument and eventually gets the manifest from that and returns it
func (da *PackageArchive) DownloadArchiveInfo(tracer trace.Tracer, packageName string, version string) (string, error) {
	trace := tracer.BeginSection("Downloading document archive info")
	defer trace.End()

	key := archive.FormKey(packageName, version)
	trace.AppendInfof("Key: %v", key)
	if da.localDocuments[key] == nil {
		da.localDocuments[key] = &localDocument{}
	}

	var cachedDocumentHash string
	var err error
	versionPtr := &version
	if version == "" {
		versionPtr = nil
	}
	MaxDelayBeforeCall := 15 //seconds
	// random back off before GetDocument call
	time.Sleep(time.Duration(getRandomBackOffTime(MaxDelayBeforeCall)*(da.timeUnit)) * time.Millisecond)
	descDocResponse, err := da.facadeClient.DescribeDocument(
		&ssm.DescribeDocumentInput{
			Name:        &packageName,
			VersionName: versionPtr,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve package document description: %v", err)
	}

	if descDocResponse == nil || descDocResponse.Document == nil {
		return "", fmt.Errorf("Failed to retreive document description for package installation")
	}

	if *descDocResponse.Document.Status != ssm.DocumentStatusActive {
		return "", fmt.Errorf("package document is not currently active, kindly retry when the status is active. Current document status - %v", *descDocResponse.Document.Status)
	}

	if descDocResponse.Document.Name == nil || *descDocResponse.Document.Name == "" {
		return "", fmt.Errorf("document name cannot be empty")
	}
	if descDocResponse.Document.DocumentVersion == nil {
		return "", fmt.Errorf("document version cannot be nil")
	}

	documentDesc := descDocResponse.Document
	da.localDocuments[key].docHash = *documentDesc.Hash

	// DescribeDocument returns the Name of the document if it belongs to the account of the instance.
	// If it is a shared document, DescribeDocument returns the document ARN as Name
	da.localDocuments[key].documentArn = *documentDesc.Name
	da.localDocuments[key].docVersion = *documentDesc.DocumentVersion

	// Read the cached manifest hash file
	hashData, err := da.manifestCache.ReadManifestHash(packageName, da.localDocuments[key].docVersion)
	if err != nil {
		// if manifest hash does not exist, getDocument to download the manifest
		return da.getDocument(packageName, version)
	}
	// if no error, proceed to describe document
	cachedDocumentHash = string(hashData)

	if cachedDocumentHash != da.localDocuments[key].docHash {
		// If the hash value of the cached hash and the document hash isn't the same, then getDocument to download the content
		return da.getDocument(packageName, version)
	} else {
		// If the hash matches, read the cached manifest. If there is an error reading the manifest, because
		// it does not exist or is corrupted, getDocument to download the manifest
		manifestData, err := da.manifestCache.ReadManifest(da.localDocuments[key].documentArn, da.localDocuments[key].docVersion)
		if err != nil || string(manifestData) == "" {
			return da.getDocument(packageName, version)
		}

		// if hash of stored anifest is the same
		// Store manifests separately for document type packages
		return string(manifestData), nil
	}
}

// ReadManifestFromCache reads the manifest that was stored in manifestCache, if present
// Document packages store the manifest with the document version
func (da *PackageArchive) ReadManifestFromCache(packageName string, version string) (*birdwatcher.Manifest, error) {
	key := archive.FormKey(packageName, version)
	if _, ok := da.localDocuments[key]; !ok {
		return nil, fmt.Errorf("Cannot find local document mapping. package name: %v, version: %v", packageName, version)
	}

	data, err := da.manifestCache.ReadManifest(da.localDocuments[key].documentArn, da.localDocuments[key].docVersion)
	if err != nil {
		return nil, err
	}

	return archive.ParseManifest(&data)
}

// WriteManifestToCache stores the manifest in manifestCache
func (da *PackageArchive) WriteManifestToCache(packageName string, version string, manifest []byte) error {
	key := archive.FormKey(packageName, version)
	if _, ok := da.localDocuments[key]; !ok {
		return fmt.Errorf("Cannot find local document mapping. package name: %v, version: %v", packageName, version)
	}

	return da.manifestCache.WriteManifest(da.localDocuments[key].documentArn, da.localDocuments[key].docVersion, manifest)
}

// helpers
func (da *PackageArchive) generateAndSaveManifestHash(name, manifest, documentVersion string) error {
	var hashChannel = make(chan []byte, 1)
	hash := sha256.Sum256([]byte(manifest))
	hashChannel <- hash[:]
	return da.manifestCache.WriteManifestHash(name, documentVersion, <-hashChannel)
}

func (da *PackageArchive) getDocument(packageName, version string) (manifest string, err error) {
	key := archive.FormKey(packageName, version)
	if _, ok := da.localDocuments[key]; !ok {
		return "", fmt.Errorf("Cannot find local document mapping. package name: %v, version: %v", packageName, version)
	}

	versionPtr := &version
	if version == "" {
		versionPtr = nil
	}
	getDocResponse, err := da.facadeClient.GetDocument(
		&ssm.GetDocumentInput{
			Name:        &packageName,
			VersionName: versionPtr,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve package document: %v", err)
	}

	if getDocResponse == nil {
		return "", fmt.Errorf("Failed to retreive document for package installation")
	}

	if *getDocResponse.Status != ssm.DocumentStatusActive {
		return "", fmt.Errorf("package document is not currently active, kindly retry when the status is active. Current document status - %v", *getDocResponse.Status)
	}
	// GetDocument returns the Name of the document if it belongs to the account of the instance.
	// If it is a shared document, GetDocument returns the document ARN as Name
	if getDocResponse.Name == nil || *getDocResponse.Name == "" {
		return "", fmt.Errorf("document name cannot be empty")
	}

	if getDocResponse.Content == nil || *getDocResponse.Content == "" {
		return "", fmt.Errorf("failed to retrieve manifest from package document")
	}

	if getDocResponse.DocumentVersion == nil {
		return "", fmt.Errorf("document version cannot be nil")
	}
	//TODO: add check for docVersion
	da.localDocuments[key].attachments = getDocResponse.AttachmentsContent
	da.localDocuments[key].docVersion = *getDocResponse.DocumentVersion
	da.localDocuments[key].documentArn = *getDocResponse.Name

	err = da.generateAndSaveManifestHash(packageName, *getDocResponse.Content, da.localDocuments[key].docVersion)
	return *getDocResponse.Content, err
}

func getRandomBackOffTime(timeInSeconds int) int {
	rand.Seed(time.Now().UnixNano())
	delay := rand.Intn(timeInSeconds)

	return delay
}
