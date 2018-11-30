// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package localpackages implements the local storage for packages managed by the ConfigurePackage plugin.
package localpackages

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filelock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/ssminstaller"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

// DownloadDelegate is a function that downloads a package to a directory provided by the repository
type DownloadDelegate func(tracer trace.Tracer, targetDirectory string) error

// InstallState is an enum describing the installation state of a package
type InstallState uint

// NOTE: Do not change the order of this enum - the numeric value is serialized as package state and must deserialize to the same value
const (
	None              InstallState = iota // Package version not present in repository
	Unknown           InstallState = iota // Present in repository but no state information or corrupt state
	Failed            InstallState = iota // Installation of the package version was attempted but failed
	Uninstalling      InstallState = iota // Package version being uninstalled version but not yet uninstalled
	Uninstalled       InstallState = iota // Successfully uninstalled version of a package (but not yet deleted)
	New               InstallState = iota // Present in the repository but not yet installed
	Upgrading         InstallState = iota // Uninstalling previous version
	Installing        InstallState = iota // Package version being installed but not yet installed
	Installed         InstallState = iota // Successfully installed version of a package
	RollbackUninstall InstallState = iota // Uninstalling as part of rollback
	RollbackInstall   InstallState = iota // Installing as part of rollback
)

// String returns the string representation of the InstallState
func (state InstallState) String() string {
	stateNames := [...]string{
		"None",
		"Unknown",
		"Failed",
		"Uninstalling",
		"Uninstalled",
		"New",
		"Upgrading",
		"Installing",
		"Installed",
		"RollbackUninstall",
		"RollbackInstall"}

	if state < None || state > RollbackInstall {
		return "StateNotFound"
	}

	return stateNames[state]
}

// Repository represents local storage for packages managed by configurePackage
// Different formats for different versions are managed within the Repository abstraction
type Repository interface {
	GetInstalledVersion(tracer trace.Tracer, packageArn string) string
	ValidatePackage(tracer trace.Tracer, packageArn string, version string) error
	RefreshPackage(tracer trace.Tracer, packageArn string, version string, packageServiceName string, downloader DownloadDelegate) error
	AddPackage(tracer trace.Tracer, packageArn string, version string, packageServiceName string, downloader DownloadDelegate) error
	SetInstallState(tracer trace.Tracer, packageArn string, version string, state InstallState) error
	GetInstallState(tracer trace.Tracer, packageArn string) (state InstallState, version string)
	RemovePackage(tracer trace.Tracer, packageArn string, version string) error
	GetInventoryData(log log.T) []model.ApplicationData
	GetInstaller(tracer trace.Tracer, configuration contracts.Configuration, packageArn string, version string) installer.Installer

	LockPackage(tracer trace.Tracer, packageArn string, action string) error
	UnlockPackage(tracer trace.Tracer, packageArn string)

	ReadManifest(packageArn string, packageVersion string) ([]byte, error)
	WriteManifest(packageArn string, packageVersion string, content []byte) error
	ReadManifestHash(packageArn string, documentVersion string) ([]byte, error)
	WriteManifestHash(packageArn string, documentVersion string, content []byte) error

	LoadTraces(tracer trace.Tracer, packageArn string) error
	PersistTraces(tracer trace.Tracer, packageArn string) error
}

// NewRepository is the factory method for the package repository with default file system dependencies
func NewRepository() Repository {
	return &localRepository{
		filesysdep:        &fileSysDepImp{},
		repoRoot:          appconfig.PackageRoot,
		lockRoot:          appconfig.PackageLockRoot,
		manifestCachePath: appconfig.ManifestCacheDirectory,
		fileLocker:        filelock.NewFileLocker(),
	}
}

// PackageInstallState represents the json structure of the current package state
type PackageInstallState struct {
	Name                 string       `json:"name"`
	Version              string       `json:"version"`
	State                InstallState `json:"state"`
	Time                 time.Time    `json:"time"`
	LastInstalledVersion string       `json:"lastinstalledversion"`
	RetryCount           int          `json:"retrycount"`
}

// PackageManifest represents json structure of package's online configuration file.
type PackageManifest struct {
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	Version         string `json:"version"`
	AppName         string `json:"appname"`         // optional inventory attribute
	AppPublisher    string `json:"apppublisher"`    // optional inventory attribute
	AppReferenceURL string `json:"appreferenceurl"` // optional inventory attribute
	AppType         string `json:"apptype"`         // optional inventory attribute
}

type localRepository struct {
	filesysdep        FileSysDep
	repoRoot          string
	lockRoot          string
	manifestCachePath string
	fileLocker        filelock.FileLocker
}

func (repo *localRepository) LockPackage(tracer trace.Tracer, packageArn string, action string) error {
	err := fileutil.MakeDirs(repo.lockRoot)
	if err != nil {
		return err
	}
	lockPath := repo.getLockPath(packageArn)
	return lockPackage(repo.fileLocker, lockPath, packageArn, action)
}

func (repo *localRepository) UnlockPackage(tracer trace.Tracer, packageArn string) {
	lockPath := repo.getLockPath(packageArn)
	unlockPackage(repo.fileLocker, lockPath, packageArn)
}

// GetInstaller returns an Installer appropriate for the package and version
// The configuration should include the appropriate OutputS3KeyPrefix for documents run by the Installer
func (repo *localRepository) GetInstaller(tracer trace.Tracer,
	configuration contracts.Configuration,
	packageArn string,
	version string) installer.Installer {

	// Give each version an independent orchestration directory to support install and uninstall for two versions during rollback
	configuration.OrchestrationDirectory = filepath.Join(configuration.OrchestrationDirectory, normalizeDirectory(version))
	return ssminstaller.New(packageArn,
		version,
		repo.getPackageVersionPath(tracer, packageArn, version),
		configuration,
		&envdetect.CollectorImp{})
}

// GetInstalledVersion returns the version of the last successfully installed package
func (repo *localRepository) GetInstalledVersion(tracer trace.Tracer, packageArn string) string {
	packageState := repo.loadInstallState(repo.filesysdep, tracer, packageArn)
	if packageState.State == Installed || (packageState.State == Unknown && packageState.LastInstalledVersion == "") {
		return packageState.Version
	} else {
		return packageState.LastInstalledVersion
	}
}

func (repo *localRepository) checkPackageIsSupported(tracer trace.Tracer, packageArn string, version string) error {
	validatetrace := tracer.BeginSection("isPackageSupported")
	defer validatetrace.End()

	path := repo.getPackageVersionPath(tracer, packageArn, version)
	if repo.filesysdep.Exists(filepath.Join(path, "install.ps1")) {
		return nil
	}
	if repo.filesysdep.Exists(filepath.Join(path, "install.sh")) {
		return nil
	}

	err := fmt.Errorf("Package is not supported (package is missing install action)")
	validatetrace.WithError(err).End()
	return err
}

// ValidatePackage returns an error if the given package version artifacts are missing, incomplete, or corrupt
func (repo *localRepository) ValidatePackage(tracer trace.Tracer, packageArn string, version string) error {
	// Find and parse manifest
	trace := tracer.BeginSection("Validate Package")

	if _, err := repo.openPackageManifest(tracer, repo.filesysdep, packageArn, version); err != nil {
		trace.WithError(err).End()
		return fmt.Errorf("Package manifest is invalid: %v", err)
	}

	hasContent := false

	packageVersionPath := repo.getPackageVersionPath(tracer, packageArn, version)
	trace.AppendDebugf("package version path for package %v version %v is %v", packageArn, version, packageVersionPath)

	files, errFiles := repo.filesysdep.GetFileNames(packageVersionPath)
	if errFiles != nil {
		trace.WithError(errFiles)
	} else {
		trace.AppendDebugf("%v files exist in %v ", len(files), packageVersionPath)
	}

	dirs, errDirs := repo.filesysdep.GetDirectoryNames(packageVersionPath)
	if errDirs != nil {
		trace.WithError(errDirs)
	} else {
		trace.AppendDebugf("%v folders exist in %v", len(dirs), packageVersionPath)
	}

	// Ensure that at least one other file or folder is present
	if errFiles == nil && len(files) > 1 {
		hasContent = true
	} else if errDirs == nil && len(dirs) > 0 {
		hasContent = true
	}

	if !hasContent {
		return fmt.Errorf("Package is incomplete")
	}

	// This is necessary to make sure pre-birdwatcher packages are deemed unsupported, triggering package refresh to birdwatched version of the package.
	if err := repo.checkPackageIsSupported(tracer, packageArn, version); err != nil {
		trace.WithError(err).End()
		return err
	}

	trace.End()
	return nil
}

// RefreshPackage updates the package binaries.  Used if ValidatePackage returns an error, initially same implementation as AddPackage
func (repo *localRepository) RefreshPackage(tracer trace.Tracer, packageArn string, version string, packageServiceName string, downloader DownloadDelegate) error {
	return repo.AddPackage(tracer, packageArn, version, packageServiceName, downloader)
}

// AddPackage creates an entry in the repository and downloads artifacts for a package
func (repo *localRepository) AddPackage(tracer trace.Tracer, packageArn string, version string, packageServiceName string, downloader DownloadDelegate) error {
	packagePath := repo.getPackageVersionPath(tracer, packageArn, version)
	if err := repo.filesysdep.MakeDirExecute(packagePath); err != nil {
		return err
	}
	if err := downloader(tracer, packagePath); err != nil {
		// If the downloader delegate execution has any errors, we should clear up the newly made directory.
		cleanupTrace := tracer.BeginSection(fmt.Sprintf("cleaning up package version path: %s", packagePath))
		if cleanupErr := repo.filesysdep.RemoveAll(packagePath); cleanupErr != nil {
			cleanupTrace.WithError(cleanupErr)
		}
		cleanupTrace.End()

		return err
	}
	// if no previous version, set state to new
	repo.SetInstallState(tracer, packageArn, version, New)

	return nil
}

// SetInstallState flags the state of a version of a package downloaded to the repository for installation
func (repo *localRepository) SetInstallState(tracer trace.Tracer, packageArn string, version string, state InstallState) error {
	var packageState = repo.loadInstallState(repo.filesysdep, tracer, packageArn)
	if state == New && packageState.State != None {
		return nil
	}
	packageState.Version = version
	packageState.Time = time.Now()
	if packageState.State == state {
		packageState.RetryCount++
	} else {
		packageState.RetryCount = 0
	}
	packageState.State = state
	if state == Installed {
		packageState.LastInstalledVersion = version
	}
	if state == Uninstalled {
		packageState.LastInstalledVersion = ""
	}

	var installStateContent string
	var err error
	if installStateContent, err = jsonutil.Marshal(packageState); err != nil {
		return err
	}
	return repo.filesysdep.WriteFile(repo.getInstallStatePath(packageArn), installStateContent)
}

// GetInstallState returns the current state of a package
func (repo *localRepository) GetInstallState(tracer trace.Tracer, packageArn string) (state InstallState, version string) {
	installState := repo.loadInstallState(repo.filesysdep, tracer, packageArn)
	return installState.State, installState.Version
}

// RemovePackage deletes an entry in the repository and removes package artifacts
func (repo *localRepository) RemovePackage(tracer trace.Tracer, packageArn string, version string) error {
	return repo.filesysdep.RemoveAll(repo.getPackageVersionPath(tracer, packageArn, version))
}

// GetInventoryData returns ApplicationData for every successfully and currently installed package in the repository
// that has inventory fields in its manifest
func (repo *localRepository) GetInventoryData(log log.T) []model.ApplicationData {
	result := make([]model.ApplicationData, 0)

	// Search package root for packages that are installed and return data from the manifest of the installed version
	var dirs []string
	var err error

	tracer := trace.NewTracer(log) // temporarily wrap log into tracer to pass forward to other method calls
	defer tracer.BeginSection("GetInventoryData").EndWithError(&err)

	if dirs, err = repo.filesysdep.GetDirectoryNames(repo.repoRoot); err != nil {
		return result
	}

	for _, packageDirectoryName := range dirs {
		var packageState *PackageInstallState
		if packageState = repo.loadInstallStateByDirectoryName(repo.filesysdep, tracer, packageDirectoryName); packageState == nil || packageState.State != Installed {
			continue
		}
		// NOTE: We could put inventory info in the installstate file.  That might be simpler than opening two files in this method.
		var manifest *PackageManifest
		manifest, err = repo.openPackageManifest(tracer, repo.filesysdep, packageState.Name, packageState.Version)
		if hasInventoryData(manifest) {
			result = append(result, createApplicationData(manifest, packageState))
		}
	}

	return result
}

// manifest cache

// filePath will return the manifest file path for a package name and package version
func (r *localRepository) filePath(packageArn string, packageVersion string) string {
	return filepath.Join(r.manifestCachePath, fmt.Sprintf("%s_%s.json", normalizeDirectory(packageArn), normalizeDirectory(packageVersion)))
}

// ReadManifest will return the manifest data for a given package name and package version from the cache
func (r *localRepository) ReadManifest(packageArn string, packageVersion string) ([]byte, error) {
	return r.filesysdep.ReadFile(r.filePath(packageArn, packageVersion))
}

// WriteManifest will put the manifest data for a given package name and package version into the cache
func (r *localRepository) WriteManifest(packageArn string, packageVersion string, content []byte) error {
	err := fileutil.MakeDirs(r.manifestCachePath)
	if err != nil {
		return err
	}
	return r.filesysdep.WriteFile(r.filePath(packageArn, packageVersion), string(content))
}

// manifest cache hash
// textPath will return the manifest file path for a package name and package version
func (r *localRepository) textPath(packageArn string, documentVersion string) string {
	return filepath.Join(r.manifestCachePath, fmt.Sprintf("%s_%s.txt", normalizeDirectory(packageArn), documentVersion))
}

// ReadManifestHash will return the manifest data for a given package name and package version from the cache
func (r *localRepository) ReadManifestHash(packageArn string, documentVersion string) ([]byte, error) {
	return r.filesysdep.ReadFile(r.textPath(packageArn, documentVersion))
}

// WriteManifestHash will put the manifest data for a given package name and package version into the cache
func (r *localRepository) WriteManifestHash(packageArn string, documentVersion string, content []byte) error {
	err := fileutil.MakeDirs(r.manifestCachePath)
	if err != nil {
		return err
	}
	return r.filesysdep.WriteFile(r.textPath(packageArn, documentVersion), string(content))
}

// hasInventoryData determines if a package should be reported to inventory by the repository
// if false, it is assumed that the package used an installer type that is already collected by inventory
func hasInventoryData(manifest *PackageManifest) bool {
	return manifest != nil && (manifest.Name != "" || manifest.AppName != "" || manifest.AppPublisher != "" || manifest.AppType != "" || manifest.AppReferenceURL != "")
}

// createApplicationData creates an ApplicationData item from a package manifest
func createApplicationData(manifest *PackageManifest, packageState *PackageInstallState) model.ApplicationData {
	var compType model.ComponentType
	if manifest.AppPublisher == "" || strings.HasPrefix(strings.ToLower(manifest.AppPublisher), "amazon") {
		compType = model.AWSComponent
	}
	appName := manifest.Name
	if manifest.AppName != "" {
		appName = manifest.AppName
	}
	return model.ApplicationData{
		Name:            appName,
		Publisher:       manifest.AppPublisher,
		Version:         manifest.Version,
		InstalledTime:   packageState.Time.Format(time.RFC3339),
		ApplicationType: manifest.AppType,
		Architecture:    model.FormatArchitecture(manifest.Architecture),
		URL:             manifest.AppReferenceURL,
		CompType:        compType,
	}
}

// getPackageRoot is a helper function that given a package's directory name returns the path to the folder containing all versions of a package
func (repo *localRepository) getPackageRootByDirectoryName(directoryName string) string {
	return filepath.Join(repo.repoRoot, directoryName)
}

// getPackageRoot is a helper function that given a packageArn returns the path to the folder containing all versions of a package
func (repo *localRepository) getPackageRoot(packageArn string) string {
	return repo.getPackageRootByDirectoryName(normalizeDirectory(packageArn))
}

// getLockPath is a helper function that builds the path to the install state file
func (repo *localRepository) getLockPath(packageArn string) string {
	return filepath.Join(repo.lockRoot, normalizeDirectory(packageArn)+".lockfile")
}

// getInstallStatePath is a helper function that given a packagearn builds the path to the install state file
func (repo *localRepository) getInstallStatePath(packageArn string) string {
	return filepath.Join(repo.getPackageRoot(packageArn), "installstate")
}

// getInstallStatePath is a helper function that given a package directory name builds the path to the install state file
func (repo *localRepository) getInstallStatePathByDirectoryName(directoryName string) string {
	return filepath.Join(repo.getPackageRootByDirectoryName(directoryName), "installstate")
}

// getPackageVersionPath is a helper function that builds a path to the directory containing the given version of a package
func (repo *localRepository) getPackageVersionPath(tracer trace.Tracer, packageArn string, version string) string {
	return filepath.Join(repo.getPackageRoot(packageArn), normalizeDirectory(version))
}

// getManifestPath is a helper function that builds the path to the manifest file for a given version of a package
func (repo *localRepository) getManifestPath(tracer trace.Tracer, packageArn string, version string, manifestName string) string {
	return filepath.Join(repo.getPackageVersionPath(tracer, packageArn, version), fmt.Sprintf("%v.json", manifestName))
}

func (repo *localRepository) getTracesPath(packageArn string) string {
	return filepath.Join(repo.getPackageRoot(packageArn), "traces")
}

// loadInstallState loads the existing installstate file or returns an appropriate default state
func (repo *localRepository) loadInstallState(filesysdep FileSysDep, tracer trace.Tracer, packageArn string) *PackageInstallState {
	var filePath = repo.getInstallStatePath(packageArn)
	return repo.parseInstallState(tracer, filesysdep, filePath, packageArn)
}

// loadInstallState loads the existing installstate file given a package folder name or returns an appropriate default state
func (repo *localRepository) loadInstallStateByDirectoryName(filesysdep FileSysDep, tracer trace.Tracer, packageDirectoryName string) *PackageInstallState {
	var filePath = repo.getInstallStatePathByDirectoryName(packageDirectoryName)
	return repo.parseInstallState(tracer, filesysdep, filePath, "")
}

// parseInstallState parses the installState file
func (repo *localRepository) parseInstallState(tracer trace.Tracer, filesysdep FileSysDep, filePath string, packageArn string) *PackageInstallState {
	packageState := PackageInstallState{Name: packageArn, State: None}
	var fileContent []byte
	var err error
	if !filesysdep.Exists(filePath) {
		if dirs, err := filesysdep.GetDirectoryNames(repo.getPackageRoot(packageArn)); err == nil && len(dirs) > 0 {
			// For pre-repository packages, this will be the case, they should be updated and validated
			return &PackageInstallState{Name: packageArn, Version: dirs[len(dirs)-1], State: Unknown}
		}
		return &PackageInstallState{Name: packageArn, State: None}
	}
	if fileContent, err = filesysdep.ReadFile(filePath); err != nil {
		return &PackageInstallState{Name: packageArn, State: Unknown}
	}
	if err = jsonutil.Unmarshal(string(fileContent[:]), &packageState); err != nil {
		tracer.CurrentTrace().AppendErrorf("InstallState file for package %v is invalid: %v", packageArn, err)
		return &PackageInstallState{Name: packageArn, State: Unknown}
	}
	return &packageState
}

// openPackageManifest returns the valid manifest or validation error for a given package version
func (repo *localRepository) openPackageManifest(tracer trace.Tracer, filesysdep FileSysDep, packageArn string, version string) (manifest *PackageManifest, err error) {
	trace := tracer.BeginSection("Validate Package")
	manifestPath := repo.getManifestPath(tracer, packageArn, version, "manifest")

	if filesysdep.Exists(manifestPath) {
		return parsePackageManifest(tracer, filesysdep, manifestPath, packageArn, version)
	}

	trace.End()
	return &PackageManifest{}, nil
}

func (repo *localRepository) LoadTraces(tracer trace.Tracer, packageArn string) error {
	tracesPath := repo.getTracesPath(packageArn)
	if !repo.filesysdep.Exists(tracesPath) {
		return nil
	}
	serialized, err := repo.filesysdep.ReadFile(tracesPath)
	_ = repo.filesysdep.RemoveAll(tracesPath) // always remove the file, even if it is corrupted
	if err != nil {
		return err
	}
	var traces []*trace.Trace
	if err = json.Unmarshal([]byte(serialized), &traces); err != nil {
		return err
	}

	tracer.PrependTraces(traces)
	return nil
}

func (repo *localRepository) PersistTraces(tracer trace.Tracer, packageArn string) error {
	serialized, err := json.Marshal(tracer.Traces())
	if err != nil {
		return err
	}
	return repo.filesysdep.WriteFile(repo.getTracesPath(packageArn), string(serialized))
}

// parsePackageManifest parses the manifest to ensure it is valid.
func parsePackageManifest(tracer trace.Tracer, filesysdep FileSysDep, filePath string, packageArn string, version string) (parsedManifest *PackageManifest, err error) {
	// load specified file from file system
	trace := tracer.BeginSection("Parse Package Manifest")
	var result = []byte{}

	if result, err = filesysdep.ReadFile(filePath); err != nil {
		trace.WithError(err).End()
		return nil, err
	}

	// parse package's JSON configuration file
	if err = json.Unmarshal(result, &parsedManifest); err != nil {
		trace.WithError(err).End()
		return nil, err
	}

	// ensure manifest conforms to defined schema
	if err = validatePackageManifest(parsedManifest, packageArn, version); err != nil {
		trace.WithError(err).End()
		return parsedManifest, err
	}

	trace.End()
	return parsedManifest, nil
}

// validatePackageManifest ensures all the fields are provided.
func validatePackageManifest(parsedManifest *PackageManifest, packageArn string, version string) error {
	// ensure non-empty and properly formatted required fields
	if parsedManifest.Name == "" {
		return fmt.Errorf("empty package name")
	} else {
		manifestName := parsedManifest.Name
		if !strings.EqualFold(manifestName, packageArn) && !strings.HasSuffix(packageArn, manifestName) {
			return fmt.Errorf("manifest name (%v) does not match expected package name (%v)", manifestName, packageArn)
		}
	}
	if parsedManifest.Version == "" {
		return fmt.Errorf("empty package version")
	} else {
		manifestVersion := parsedManifest.Version
		if !strings.EqualFold(manifestVersion, version) {
			return fmt.Errorf("manifest version (%v) does not match expected package version (%v)", manifestVersion, version)
		}
	}

	return nil
}
