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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

// DownloadDelegate is a function that downloads a package to a directory provided by the repository
type DownloadDelegate func(targetDirectory string) error

// InstallState is an enum describing the installation state of a package
type InstallState uint

const (
	None         InstallState = iota // Package version not present in repository
	Unknown      InstallState = iota // Present in repository but no state information or corrupt state
	Failed       InstallState = iota // Installation of the package version was attempted but failed
	Uninstalling InstallState = iota // Package version being uninstalled version but not yet uninstalled
	Uninstalled  InstallState = iota // Successfully uninstalled version of a package (but not yet deleted)
	New          InstallState = iota // Present in the repository but not yet installed
	Upgrading    InstallState = iota // Uninstalling previous version
	Installing   InstallState = iota // Package version being installed but not yet installed
	Installed    InstallState = iota // Successfully installed version of a package
)

// Repository represents local storage for packages managed by configurePackage
// Different formats for different versions are managed within the Repository abstraction
type Repository interface {
	GetInstalledVersion(context context.T, packageName string) string
	ValidatePackage(context context.T, packageName string, version string) error
	RefreshPackage(context context.T, packageName string, version string, downloader DownloadDelegate) error
	AddPackage(context context.T, packageName string, version string, downloader DownloadDelegate) error
	SetInstallState(context context.T, packageName string, version string, state InstallState) error
	GetInstallState(context context.T, packageName string) (state InstallState, version string)
	GetAction(context context.T, packageName string, version string, actionName string) (exists bool, actionDocument []byte, workingDir string, err error)
	RemovePackage(context context.T, packageName string, version string) error
	GetInventoryData(context context.T) []model.ApplicationData
}

// NewRepositoryDefault is the factory method for the package repository with default file system dependencies
func NewRepositoryDefault() Repository {
	return &localRepository{filesysdep: &fileSysDepImp{}, repoRoot: appconfig.PackageRoot}
}

// TODO:MF: When we have unit tests specifically for the repository and are using the mock repository in
//   configurePackage tests, it should be possible to make this filesysdep a static dep instead of an instance parameter
//   (like other deps) but need to consider how to provide access to the repository root

// NewRepository is the factory method for the package repository
func NewRepository(fileSystemDependencies FileSysDep, repositoryRoot string) Repository {
	return &localRepository{filesysdep: fileSystemDependencies, repoRoot: repositoryRoot}
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
	filesysdep FileSysDep
	repoRoot   string
}

// GetInstalledVersion returns the version of the last successfully installed package
func (repo *localRepository) GetInstalledVersion(context context.T, packageName string) string {
	packageState := repo.loadInstallState(repo.filesysdep, context, packageName)
	if packageState.State == Installed || (packageState.State == Unknown && packageState.LastInstalledVersion == "") {
		return packageState.Version
	} else {
		return packageState.LastInstalledVersion
	}
}

// ValidatePackage returns an error if the given package version artifacts are missing, incomplete, or corrupt
func (repo *localRepository) ValidatePackage(context context.T, packageName string, version string) error {
	// Find and parse manifest
	if _, err := repo.openPackageManifest(repo.filesysdep, packageName, version); err != nil {
		return fmt.Errorf("Package manifest is invalid: %v", err)
	}
	// Ensure that at least one other file or folder is present
	if files, err := repo.filesysdep.GetFileNames(repo.getPackageVersionPath(packageName, version)); err == nil && len(files) > 1 {
		return nil
	}
	if dirs, err := repo.filesysdep.GetDirectoryNames(repo.getPackageVersionPath(packageName, version)); err == nil && len(dirs) > 0 {
		return nil
	}
	return fmt.Errorf("Package manifest exists, but all other content is missing")
}

// RefreshPackage updates the package binaries.  Used if ValidatePackage returns an error, initially same implementation as AddPackage
func (repo *localRepository) RefreshPackage(context context.T, packageName string, version string, downloader DownloadDelegate) error {
	return repo.AddPackage(context, packageName, version, downloader)
}

// AddPackage creates an entry in the repository and downloads artifacts for a package
func (repo *localRepository) AddPackage(context context.T, packageName string, version string, downloader DownloadDelegate) error {
	packagePath := repo.getPackageVersionPath(packageName, version)
	if err := repo.filesysdep.MakeDirExecute(packagePath); err != nil {
		return err
	}
	return downloader(packagePath)
}

// SetInstallState flags the state of a version of a package downloaded to the repository for installation
func (repo *localRepository) SetInstallState(context context.T, packageName string, version string, state InstallState) error {
	var packageState = repo.loadInstallState(repo.filesysdep, context, packageName)
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
	return repo.filesysdep.WriteFile(repo.getInstallStatePath(packageName), installStateContent)
}

// GetInstallState returns the current state of a package
func (repo *localRepository) GetInstallState(context context.T, packageName string) (state InstallState, version string) {
	installState := repo.loadInstallState(repo.filesysdep, context, packageName)
	return installState.State, installState.Version
}

// GetAction returns a JSON document describing a management action (including working directory) or an empty string
// if there is nothing to do for a given action
func (repo *localRepository) GetAction(context context.T, packageName string, version string, actionName string) (exists bool, actionDocument []byte, workingDir string, err error) {
	actionPath := repo.getActionPath(packageName, version, actionName)
	if !repo.filesysdep.Exists(actionPath) {
		return false, []byte{}, "", nil
	}
	if actionContent, err := repo.filesysdep.ReadFile(actionPath); err != nil {
		return true, []byte{}, "", err
	} else {
		actionJson := string(actionContent[:])
		var jsonTest interface{}
		if err = jsonutil.Unmarshal(actionJson, &jsonTest); err != nil {
			return true, []byte{}, "", err
		}
		return true, actionContent, repo.getPackageVersionPath(packageName, version), nil
	}
}

// RemovePackage deletes an entry in the repository and removes package artifacts
func (repo *localRepository) RemovePackage(context context.T, packageName string, version string) error {
	return repo.filesysdep.RemoveAll(repo.getPackageVersionPath(packageName, version))
}

// GetInventoryData returns ApplicationData for every successfully and currently installed package in the repository
func (repo *localRepository) GetInventoryData(context context.T) []model.ApplicationData {
	result := make([]model.ApplicationData, 0)

	// Search package root for packages that are installed and return data from the manifest of the installed version
	var dirs []string
	var err error
	if dirs, err = repo.filesysdep.GetDirectoryNames(repo.repoRoot); err != nil {
		return nil
	}

	for _, packageName := range dirs {
		var packageState *PackageInstallState
		if packageState = repo.loadInstallState(repo.filesysdep, context, packageName); packageState.State != Installed {
			continue
		}
		// NOTE: We could put inventory info in the installstate file.  That might be simpler than opening two files in this method.
		var manifest *PackageManifest
		if manifest, err = repo.openPackageManifest(repo.filesysdep, packageName, packageState.Version); err != nil {
			continue
		}
		result = append(result, createApplicationData(manifest, packageState))
	}

	return result
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

// getPackageRoot is a helper function that returns the path to the folder containing all versions of a package
func (repo *localRepository) getPackageRoot(packageName string) string {
	return filepath.Join(repo.repoRoot, packageName)
}

// getInstallStatePath is a helper function that builds the path to the install state file
func (repo *localRepository) getInstallStatePath(packageName string) string {
	return filepath.Join(repo.getPackageRoot(packageName), "installstate")
}

// getPackageVersionPath is a helper function that builds a path to the directory containing the given version of a package
func (repo *localRepository) getPackageVersionPath(packageName string, version string) string {
	return filepath.Join(repo.getPackageRoot(packageName), version)
}

// getActionPath is a helper function that builds the path to an action document file
func (repo *localRepository) getActionPath(packageName string, version string, actionName string) string {
	return filepath.Join(repo.getPackageVersionPath(packageName, version), fmt.Sprintf("%v.json", actionName))
}

// getManifestPath is a helper function that builds the path to the manifest file for a given version of a package
func (repo *localRepository) getManifestPath(packageName string, version string) string {
	return filepath.Join(repo.getPackageVersionPath(packageName, version), fmt.Sprintf("%v.json", packageName))
}

// loadInstallState loads the existing installstate file or returns an appropriate default state
func (repo *localRepository) loadInstallState(filesysdep FileSysDep, context context.T, packageName string) *PackageInstallState {
	packageState := PackageInstallState{Name: packageName, State: None}
	var fileContent []byte
	var err error
	var filePath = repo.getInstallStatePath(packageName)
	if !filesysdep.Exists(filePath) {
		if dirs, err := filesysdep.GetDirectoryNames(repo.getPackageRoot(packageName)); err == nil && len(dirs) > 0 {
			// For pre-repository packages, this will be the case, they should be updated and validated
			return &PackageInstallState{Name: packageName, Version: dirs[len(dirs)-1], State: Unknown}
		}
		return &PackageInstallState{Name: packageName, State: None}
	}
	if fileContent, err = filesysdep.ReadFile(filePath); err != nil {
		return &PackageInstallState{Name: packageName, State: Unknown}
	}
	if err = jsonutil.Unmarshal(string(fileContent[:]), &packageState); err != nil {
		context.Log().Errorf("InstallState file for package %v is invalid: %v", packageName, err)
		return &PackageInstallState{Name: packageName, State: Unknown}
	}
	return &packageState
}

// openPackageManifest returns the valid manifest or validation error for a given package version
func (repo *localRepository) openPackageManifest(filesysdep FileSysDep, packageName string, version string) (manifest *PackageManifest, err error) {
	manifestPath := repo.getManifestPath(packageName, version)
	if !filesysdep.Exists(manifestPath) {
		return nil, fmt.Errorf("No manifest found for package %v, version %v", packageName, version)
	} else {
		return parsePackageManifest(filesysdep, manifestPath, packageName, version)
	}
}

// parsePackageManifest parses the manifest to ensure it is valid.
func parsePackageManifest(filesysdep FileSysDep, filePath string, packageName string, version string) (parsedManifest *PackageManifest, err error) {
	// load specified file from file system
	var result = []byte{}
	if result, err = filesysdep.ReadFile(filePath); err != nil {
		return nil, err
	}

	// parse package's JSON configuration file
	if err = json.Unmarshal(result, &parsedManifest); err != nil {
		return nil, err
	}

	// ensure manifest conforms to defined schema
	return parsedManifest, validatePackageManifest(parsedManifest, packageName, version)
}

// validatePackageManifest ensures all the fields are provided.
func validatePackageManifest(parsedManifest *PackageManifest, packageName string, version string) error {
	// ensure non-empty and properly formatted required fields
	if parsedManifest.Name == "" {
		return fmt.Errorf("empty package name")
	} else {
		manifestName := parsedManifest.Name
		if !strings.EqualFold(manifestName, packageName) {
			return fmt.Errorf("manifest name (%v) does not match expected package name (%v)", manifestName, packageName)
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
	// TODO:MF: see if we can remove platform and arch.  We don't really use them... arch shows up in inventory but does it need to for SSM Packages?  If the SSM Package installs an rpm, for example, arch will be present there.

	return nil
}
