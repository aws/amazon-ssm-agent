// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"golang.org/x/sys/windows/registry"
)

const SSMDataHardened = "SSMDataHardened"
const SSMDataHardenedRegistryValuePath = appconfig.ItemPropertyPath + SSMDataHardened

// AddHardenedFlagToRegistry adds a flag to the windows registry
// which signals %PROGRAMDATA%\Amazon\SSM has been hardened
func AddHardenedFlagToRegistry() error {
	systemKey, err := registry.OpenKey(registry.LOCAL_MACHINE, "SYSTEM", registry.CREATE_SUB_KEY)
	if err != nil {
		return fmt.Errorf("Error opening SYSTEM registry key: %v", err)
	}
	defer systemKey.Close()

	ccsetKey, alreadyExists, err := registry.CreateKey(systemKey, "CurrentControlSet", registry.CREATE_SUB_KEY)
	if err != nil && !alreadyExists {
		return fmt.Errorf("Error creating SYSTEM\\CurrentControlSet registry key: %v", err)
	}
	defer ccsetKey.Close()

	servicesKey, alreadyExists, err := registry.CreateKey(ccsetKey, "Services", registry.SET_VALUE)
	if err != nil && !alreadyExists {
		return fmt.Errorf("Error creating SYSTEM\\CurrentControlSet\\Services registry key: %v", err)
	}
	defer servicesKey.Close()

	agentKey, alreadyExists, err := registry.CreateKey(servicesKey, "AmazonSSMAgent", registry.SET_VALUE)
	if err != nil && !alreadyExists {
		return fmt.Errorf("Error creating %v registry key: %v", appconfig.ItemPropertyName, err)
	}
	defer agentKey.Close()

	if err := agentKey.SetDWordValue(SSMDataHardened, 1); err != nil {
		return fmt.Errorf("Error setting %v registry value: %v", SSMDataHardenedRegistryValuePath, err)
	}

	fmt.Println("SSMDataHardened flag added to registry")
	return nil
}

// DataDirChecksOnInitialInstall checks %PROGRAMDATA%\Amazon and scans %PROGRAMDATA%\Amazon\SSM
// Links are removed if allowLinkDeletions is true
// Step is skipped if hardened flag is set
func DataDirChecksOnInitialInstall(allowLinkDeletions bool) error {
	fmt.Println("Checking if initial install...")

	// check registry
	ssmKey, err := registry.OpenKey(registry.LOCAL_MACHINE, appconfig.ItemPropertyPath, registry.QUERY_VALUE)
	if err == nil {
		defer ssmKey.Close()

		_, _, err = ssmKey.GetIntegerValue(SSMDataHardened)

		if err != nil && err != registry.ErrNotExist {
			fmt.Println(fmt.Sprintf("Error getting registry value for %v: %v", SSMDataHardenedRegistryValuePath, err))
		} else if err != nil && err == registry.ErrNotExist {
			fmt.Println(fmt.Sprintf("Registry value %v doesn't exist", SSMDataHardenedRegistryValuePath))
		} else {
			fmt.Println(fmt.Sprintf("Skipping data directory check, %v registry value exists", SSMDataHardenedRegistryValuePath))
			return nil
		}
	} else if err != nil && err != registry.ErrNotExist {
		fmt.Println(fmt.Sprintf("Error opening %v registry key: %v", appconfig.ItemPropertyPath, err))
	} else if err != nil && err == registry.ErrNotExist {
		fmt.Println(fmt.Sprintf("Registry key %v doesn't exist", appconfig.ItemPropertyPath))
	}

	start := time.Now()

	fmt.Println("Checking %PROGRAMDATA%\\Amazon")
	if err := checkAmazonDataDirectory(); err != nil {
		return err
	}

	fmt.Println("Scanning %PROGRAMDATA%\\Amazon\\SSM")
	fileLinks, dirLinks, err := scanSSMDataDirectory(allowLinkDeletions)
	if err != nil {
		return err
	}
	if allowLinkDeletions {
		if err = removeLinks(fileLinks, dirLinks); err != nil {
			return err
		}
	} else if len(fileLinks) > 0 || len(dirLinks) > 0 {
		return fmt.Errorf("Forbidden links at: %v", append(fileLinks, dirLinks...))
	}

	fmt.Println(fmt.Sprintf("Data folder scan time: %v", time.Since(start)))
	return nil
}

// checkAmazonDataDirectory returns error if Amazon directory is link
func checkAmazonDataDirectory() error {
	// check amazon data folder
	if _, err := os.Stat(appconfig.AmazonDataPath); !os.IsNotExist(err) {
		var amzDataStatInfo os.FileInfo
		var amzDataLstatInfo os.FileInfo
		if amzDataStatInfo, err = os.Stat(appconfig.AmazonDataPath); err != nil {
			return fmt.Errorf("Stat error during data folder validation at %v", appconfig.AmazonDataPath)
		}
		if amzDataLstatInfo, err = os.Lstat(appconfig.AmazonDataPath); err != nil {
			return fmt.Errorf("Lstat error during data folder validation at %v", appconfig.AmazonDataPath)
		}
		if !os.SameFile(amzDataStatInfo, amzDataLstatInfo) {
			return fmt.Errorf("Forbidden link at: %v", appconfig.AmazonDataPath)
		}
	}

	return nil
}

// scanSSMDataDirectory scans for links and attempts deletion
func scanSSMDataDirectory(allowLinkDeletions bool) ([]string, []string, error) {
	var fileLinks []string
	var dirLinks []string
	// recursively check through ssm data folder
	if _, err := os.Stat(appconfig.SSMDataPath); !os.IsNotExist(err) {
		walkFn := func(path string, dirEntry os.FileInfo, err error) error {
			var dirEntryInfo os.FileInfo
			var pathStatInfo os.FileInfo

			if err != nil {
				return fmt.Errorf("Walk error during data folder validation at %v: %v", path, err)
			}
			if dirEntry == nil {
				return nil
			}

			dirEntryInfo = dirEntry

			retries := 3
			for i := 1; i <= retries; i++ {
				if pathStatInfo, err = os.Stat(path); err != nil {
					if os.IsNotExist(err) {
						fmt.Printf("Skipping stat at %v, path no longer exists\n", path)
						return nil
					} else if i == retries {
						return fmt.Errorf("stat error during data folder validation at %v: %v", path, err)
					} else {
						fmt.Printf("Stat error during data folder validation at %v: %v\n", path, err)
						time.Sleep(500 * time.Millisecond)
					}
				} else {
					break
				}
			}

			if !os.SameFile(dirEntryInfo, pathStatInfo) {
				fmt.Println(fmt.Sprintf("Forbidden link at: %v", path))
				if dirEntry.IsDir() {
					dirLinks = append([]string{path}, dirLinks...)
				} else {
					fileLinks = append([]string{path}, fileLinks...)
				}
			}
			return nil
		}

		err := filepath.Walk(appconfig.SSMDataPath, walkFn)
		if err != nil {
			return nil, nil, err
		}

	}
	return fileLinks, dirLinks, nil
}

// removeLinks deletes file and directory links
func removeLinks(invalidFiles []string, invalidDirs []string) error {
	for _, file := range invalidFiles {
		fmt.Println(fmt.Sprintf("Removing file %v", file))
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Error removing file %v: %v", file, err)
		}
	}
	for _, dir := range invalidDirs {
		fmt.Println(fmt.Sprintf("Removing dir %v", dir))
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Error removing directory %v: %v", dir, err)
		}
	}

	return nil
}
