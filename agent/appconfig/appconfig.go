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

// Package appconfig manages the configuration of the agent.
package appconfig

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// T stores agent configuration values.
type T struct {
	Profile struct {
		ProfilePath string
		ProfileName string
	}
	Mds struct {
		Endpoint            string
		CommandWorkersLimit int
		StopTimeoutMillis   int64
		CommandRetryLimit   int
	}
	Ssm struct {
		Endpoint               string
		HealthFrequencyMinutes int
	}
	Agent struct {
		Name                 string
		Version              string
		Region               string
		OrchestrationRootDir string
		DownloadRootDir      string
	}
	Os struct {
		Lang    string
		Name    string
		Version string
	}
	S3 struct {
		Region    string
		LogBucket string
		LogKey    string
	}
	Plugins map[string]interface{}
}

var loadedConfig *T
var lock sync.RWMutex

// GetConfig loads the app configuration.
// If reload is true, it loads the config afresh,
// otherwise it returns a previous loaded version, if any.
func GetConfig(reload bool) (T, error) {
	if reload || !isLoaded() {
		var config T
		path, pathErr := getAppConfigPath()
		if pathErr != nil {
			return config, pathErr
		}
		err := jsonutil.UnmarshalFile(path, &config)
		if err != nil {
			return config, err
		}
		config.Os.Name = runtime.GOOS
		config.Agent.Version = version.Version
		parser(&config)
		cache(config)

	}
	return getCached(), nil
}

func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedConfig != nil
}

func cache(config T) {
	lock.Lock()
	defer lock.Unlock()
	loadedConfig = &config
}

func getCached() T {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedConfig
}

// ProfileCredentials checks to see if specific profile is being asked to use
func (config T) ProfileCredentials() (credsInConfig *credentials.Credentials, err error) {
	// the credentials file location and profile to load
	credsInConfig = credentials.NewSharedCredentials(config.Profile.ProfilePath, config.Profile.ProfileName)
	_, err = credsInConfig.Get()
	if err != nil {
		fmt.Println("AWS credentials under user profile has not been configured, ignoring...", err)
		credsInConfig = nil
	}
	return
}

// looks for appconfig in working directory first and then the platform specific folder
func getAppConfigPath() (path string, err error) {
	if _, err = os.Stat(AppConfigWorkingDirectoryPath); err == nil {
		log.Println("Loading appconfig from working directory - ", AppConfigWorkingDirectoryPath)
		return AppConfigWorkingDirectoryPath, err
	}

	log.Println("Unable to find appconfig at ", AppConfigWorkingDirectoryPath)

	// looking for appconfig in the platform specific folder
	if _, err = os.Stat(AppConfigPath); err == nil {
		log.Println("Loading appconfig from ", AppConfigPath)
		return AppConfigPath, err
	}

	log.Println("Unable to find appconfig at ", AppConfigPath)
	return "", err
}
