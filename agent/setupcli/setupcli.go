// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package main represents the entry point of the ssm agent setup manager.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/configurationmanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/cihub/seelog"
)

var LogMutex = new(sync.RWMutex)

// cli parameters
var artifactsDir string
var region string
var install bool
var local bool
var shutdown bool
var register bool
var role string
var tags string
var override bool
var help bool

var getPackageManager = managers.GetPackageManager
var getConfigurationManager = managers.GetConfigurationManager
var getServiceManager = managers.GetServiceManager
var getRegisterManager = managers.GetRegisterManager
var getRegistrationInfo = registration.NewOnpremRegistrationInfo
var osExit = func(exitCode int, log log.T, message string, messageArgs ...interface{}) {
	if message != "" {
		if exitCode == 0 {
			log.Infof(message, messageArgs...)
		} else {
			log.Errorf(message, messageArgs...)
		}
	}
	log.Flush()
	log.Close()
	os.Exit(exitCode)
}

func main() {
	var packageManager packagemanagers.IPackageManager
	var serviceManager servicemanagers.IServiceManager
	var err error

	log := initializeLogger()
	setParams(log)
	verifyParams(log)

	if packageManager, err = getPackageManager(log); err != nil {
		osExit(1, log, "Failed to determine package manager: %v", err)
	}
	if serviceManager, err = getServiceManager(log); err != nil {
		osExit(1, log, "Failed to determine service manager: %v", err)
	}

	// download and install
	if install {
		// Configure ssm agent using configuration in artifacts folder if not already configured
		configManager := getConfigurationManager()
		log.Infof("Attempting to configure agent")
		if err = configurationmanager.ConfigureAgent(log, configManager, artifactsDir); err != nil {
			log.Warnf("Failed to configure agent with error: %v", err)
		}

		log.Info("Starting amazon-ssm-agent install")
		if isInstalled, err := packageManager.IsAgentInstalled(); err != nil {
			osExit(1, log, "Failed to determine if agent is installed: %v", err)
		} else if isInstalled {
			log.Infof("Found existing agent installation, uninstalling current agent for potential upgrade")
			// TODO: Check agent version to determine if uninstall/install is required, introduce update flag for updates
			if err = packagemanagers.UninstallAgent(packageManager); err != nil {
				osExit(1, log, "Failed to uninstall the agent: %v", err)
			}
		}

		log.Infof("Starting agent installation")
		if err := packagemanagers.InstallAgent(packageManager, serviceManager, artifactsDir); err != nil {
			osExit(1, log, "Failed to install agent: %v", err)
		}
		log.Infof("Agent installed successfully")
	}

	// register
	if register {
		log.Info("Verifying agent is installed before attempting to register")
		if isInstalled, err := packageManager.IsAgentInstalled(); err != nil {
			osExit(1, log, "Failed to determine if agent is installed: %v", err)
		} else if !isInstalled {
			osExit(1, log, "Agent must be installed before attempting to register")
		}

		log.Info("Verified agent is installed")

		registrationInfo := getRegistrationInfo()
		instanceId := registrationInfo.InstanceID(log, "", registration.RegVaultKey)

		if instanceId != "" {
			log.Infof("Agent already registered with instance id %s", instanceId)
		} else {
			log.Info("Agent is not registered")
		}

		if instanceId != "" && !override {
			log.Info("skipping registration because override flag is not set, just starting agent")
			if err = servicemanagers.StartAgent(serviceManager, log); err != nil {
				osExit(1, log, "Failed to start agent: %v", err)
			}
			return
		}

		log.Infof("Stopping agent before registering")
		if err = servicemanagers.StopAgent(serviceManager, log); err != nil {
			osExit(1, log, "Failed to stop agent: %v", err)
		}

		log.Infof("Registering agent")
		if err = getRegisterManager().RegisterAgent(region, role, tags); err != nil {
			osExit(1, log, "Failed to register agent: %v", err)
		}

		log.Infof("Successfully registered the agent, starting agent")
		if err = servicemanagers.StartAgent(serviceManager, log); err != nil {
			osExit(1, log, "Failed to start agent: %v", err)
		}

		log.Infof("Successfully started agent, reloading registration info")
		registrationInfo.ReloadInstanceInfo(log, "", registration.RegVaultKey)
		instanceId = registrationInfo.InstanceID(log, "", registration.RegVaultKey)
		if instanceId == "" {
			osExit(1, log, "Failed to get new instance id from registration info after registration")
		} else {
			log.Infof("Instance id after registration is %s", instanceId)
		}
	}

	// shutdown
	if shutdown {
		log.Info("Shutting down amazon-ssm-agent")
		if err = servicemanagers.StopAgent(serviceManager, log); err != nil {
			osExit(1, log, "Failed to shut down agent: %v", err)
		}
	}

	log.Flush()
	log.Close()
}

func setParams(log log.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.Usage = flagUsage

	flag.StringVar(&artifactsDir, "artifacts-dir", "", "")
	flag.StringVar(&region, "region", "", "")
	flag.BoolVar(&install, "install", false, "")
	flag.BoolVar(&shutdown, "shutdown", false, "")
	flag.BoolVar(&register, "register", false, "")
	flag.StringVar(&role, "role", "", "")
	flag.BoolVar(&override, "override", false, "")
	flag.StringVar(&tags, "tags", "", "")
	flag.BoolVar(&help, "help", false, "")

	flag.Parse()

	// Environment variable overrides
	if os.Getenv("SSM_ARTIFACTS_PATH") != "" {
		artifactsDir = os.Getenv("SSM_ARTIFACTS_PATH")
	}

	if os.Getenv("AWS_REGION") != "" {
		region = os.Getenv("AWS_REGION")
	}

	if os.Getenv("SSM_REGISTRATION_ROLE") != "" {
		role = os.Getenv("SSM_REGISTRATION_ROLE")
	}

	if os.Getenv("SSM_RESOURCE_TAGS") != "" {
		tags = os.Getenv("SSM_RESOURCE_TAGS")
	}

	if os.Getenv("SSM_OVERRIDE_EXISTING_REGISTRATION") == "true" {
		override = true
	}

	if artifactsDir == "" {
		// If artifactsDir is not set, assume we are supported to use where the binary is located
		var err error
		artifactsDir, err = getExecutableFolderPath()
		if err != nil {
			log.Warnf("Failed to get path of executable to set artifacts dir: %v", err)
		}
	}
}

func getExecutableFolderPath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath, err := filepath.EvalSymlinks(filepath.Dir(ex))
	if err != nil {
		return "", err
	}

	return exPath, nil
}

func verifyParams(log log.T) {
	if flag.NFlag() == 0 || help {
		flagUsage()
		osExit(0, log, "")
	}

	log.Info("Setup parameters:")
	log.Infof("artifactsDir=%v", artifactsDir)
	log.Infof("region=%v", region)
	log.Infof("install=%v", install)
	log.Infof("shutdown=%v", shutdown)
	log.Infof("register=%v", register)
	log.Infof("role=%v", role)
	log.Infof("tags=%v", tags)
	log.Infof("override=%v", override)

	var errMessage string
	if region == "" {
		errMessage += "Region required. "
	}

	if artifactsDir == "" {
		errMessage += "Artifacts directory required. "
	}

	if !install && !register && !shutdown {
		errMessage += "Action required (install|register|shutdown). "
	}

	if register && role == "" {
		errMessage += "Role required for registration. "
	}

	if errMessage != "" {
		flagUsage()
		osExit(1, log, "Invalid parameters - %v", errMessage)
	}
}

func flagUsage() {
	fmt.Fprintln(os.Stderr, "\n\nCommand-line Usage:")
	fmt.Fprintln(os.Stderr, "\t-artifacts-dir \tDirectory for ssm agent install package and install/register scripts")
	fmt.Fprintln(os.Stderr, "\t-region        \tRegion used for ssm agent download location and registration")
	fmt.Fprintln(os.Stderr, "\t-download      \tDownload ssm agent install package based on platform")
	fmt.Fprintln(os.Stderr, "\t-install       \tInstall ssm agent based on platform")
	fmt.Fprintln(os.Stderr, "\t-register      \tRegister ssm agent if unregistered or override is set")
	fmt.Fprintln(os.Stderr, "\t\t-role     \tRole ssm agent will be registered with           \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t\t-override \tOverride existing registration if present        \t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\t\t-tags     \tTags to attach to ssm instance on registrations  \t(OPTIONAL)")
}

func initializeLogger() log.T {
	// log to console
	seelogConfig := `
<seelog type="sync">
	<outputs>
		<console formatid="fmtconsole"/>
	</outputs>
	<formats>
        <format id="fmtconsole" format="%LEVEL %Msg%n"/>
    </formats>
</seelog>
`
	seelogger, _ := seelog.LoggerFromConfigAsBytes([]byte(seelogConfig))
	loggerInstance := &log.DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger
	formatFilter := &log.ContextFormatFilter{Context: []string{}}
	return &log.Wrapper{Format: formatFilter, M: LogMutex, Delegate: loggerInstance}
}
