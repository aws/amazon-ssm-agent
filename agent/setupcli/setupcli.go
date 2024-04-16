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

//go:build !darwin
// +build !darwin

// Package main represents the entry point of the ssm agent setup manager.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/configurationmanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/downloadmanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/helpers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/registermanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"
	agentVersioning "github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
	utilityCmn "github.com/aws/amazon-ssm-agent/common/utility"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/cihub/seelog"
)

// cli parameters
var (
	LogMutex                = new(sync.RWMutex)
	artifactsDir            string
	region                  string
	install                 bool
	shutdown                bool
	register                bool
	role                    string
	tags                    string
	activationCode          string
	activationId            string
	environment             string
	skipSignatureValidation bool
	override                bool
	registerInputModel      *registermanager.RegisterAgentInputModel
	help                    bool
	version                 string
	downgrade               bool
	manifestUrl             string
)

var (
	getPackageManager       = managers.GetPackageManager
	getConfigurationManager = managers.GetConfigurationManager
	getServiceManager       = managers.GetServiceManager
	getRegisterManager      = managers.GetRegisterManager
	getRegistrationInfo     = registration.NewOnpremRegistrationInfo
	getVerificationManager  = managers.GetVerificationManager
	getDownloadManager      = managers.GetDownloadManager
	startAgent              = servicemanagers.StartAgent
	hasElevatedPermissions  = utilityCmn.IsRunningElevatedPermissions

	osExecutable         = os.Executable
	evalSymLinks         = filepath.EvalSymlinks
	filePathDir          = filepath.Dir
	fileUtilCreateTemp   = fileutil.CreateTempDir
	fileUtilMakeDirs     = fileutil.MakeDirs
	isPlatformNano       = platform.IsPlatformNanoServer
	utilityCheckSum      = utility.ComputeCheckSum
	newProcessExecutor   = executor.NewProcessExecutor
	svcMgrStopAgent      = servicemanagers.StopAgent
	helperInstallAgent   = helpers.InstallAgent
	helperUnInstallAgent = helpers.UninstallAgent
	timeSleep            = time.Sleep
)

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

// main function to perform SSM-Setup-CLI tasks for greengrass and On-prem devices
func main() {
	// initialization of various managers
	var packageManager packagemanagers.IPackageManager
	var serviceManager servicemanagers.IServiceManager
	var verificationManager verificationmanagers.IVerificationManager
	var err error

	// set parameters passed
	setParams()

	if strings.ToLower(environment) == string(common.GreengrassEnv) {
		// initialize logger
		log := initializeLogger()
		defer func() {
			log.Flush()
			log.Close()
		}()

		// set & verify params needed for greengrass
		setVerifyGreenGrassParams(log)

		// initialize
		if packageManager, err = getPackageManager(log); err != nil {
			osExit(1, log, "Failed to determine package manager: %v", err)
		}
		if serviceManager, err = getServiceManager(log); err != nil {
			osExit(1, log, "Failed to determine service manager: %v", err)
		}
		// performs greengrass related based on arguments
		performGreengrassSteps(log, packageManager, serviceManager)

	} else if strings.ToLower(environment) == string(common.OnPremEnv) || strings.TrimSpace(environment) == "" {
		// Check whether the SSM Setup CLI is running with elevated permissions or not
		err := hasElevatedPermissions()
		if err != nil {
			fmt.Println("Please run as root/admin. Err: ", err)
			os.Exit(1)
		}

		log := initializeLoggerForOnprem()
		defer func() {
			log.Flush()
			log.Close()
		}()
		log.Infof("ssm-setup-cli -version: %v", agentVersioning.Version)

		// set proxy values
		common.SetProxyConfig(log)

		// set & verify params needed for Onprem
		setVerifyOnpremParams(log)

		// Initialization
		if packageManager, err = getPackageManager(log); err != nil {
			osExit(1, log, "Failed to determine package manager: %v", err)
		}
		if serviceManager, err = getServiceManager(log); err != nil {
			osExit(1, log, "Failed to determine service manager: %v", err)
		}
		// verification manager will be used only by On-prem devices
		if verificationManager, err = getVerificationManager(); err != nil {
			osExit(1, log, "Failed to determine verification manager: %v", err)
		}
		// Perform on-prem steps based on flags passed
		err = performOnpremSteps(log, packageManager, verificationManager, serviceManager)
		if err != nil {
			osExit(1, log, "Failed to perform Onprem registration: %v", err)
		}

	} else {
		log := initializeLogger()
		flagUsage()
		osExit(1, log, "Invalid environment. - %v", environment)
	}
}

func performGreengrassSteps(log log.T, packageManager packagemanagers.IPackageManager, serviceManager servicemanagers.IServiceManager) {
	var err error

	// Check whether the SSM Setup CLI is running with elevated permissions or not
	err = hasElevatedPermissions()
	if err != nil {
		osExit(1, log, "ssm-setup-cli is not executed by root")
	}

	// download and install
	if install {
		// Configure ssm agent using configuration in artifacts folder if not already configured
		configManager := getConfigurationManager()
		log.Infof("Attempting to configure agent")
		if err = configurationmanager.ConfigureAgent(log, configManager, artifactsDir); err != nil {
			log.Warnf("Failed to configure agent with error: %w", err)
		}

		if err = configManager.CreateUpdateAgentConfigWithOnPremIdentity(); err != nil {
			log.Warnf("Failed to configure agent with On-prem identity: %v", err)
		}

		log.Info("Starting amazon-ssm-agent install")
		var isInstalled bool
		var reInstallAgent bool
		if isInstalled, err = packageManager.IsAgentInstalled(); err != nil {
			osExit(1, log, "Failed to determine if agent is installed: %w", err)
		} else if isInstalled {
			log.Infof("Agent already installed, checking version")
			if version, err := packageManager.GetInstalledAgentVersion(); err != nil {
				log.Warnf("Failed to get agent version, falling back to re-installation: %w", err)
				reInstallAgent = true
			} else {
				log.Infof("Agent version installed is %s", version)
				if isVersionAlreadyInstalled, err := hasAgentAlreadyInstalled(version); err != nil || !isVersionAlreadyInstalled {
					log.Warnf("Installed version is older/higher than expected Agent Version or Failed to compare, attempting to reinstall the agent: %w", err)
					reInstallAgent = true
				} else if isVersionAlreadyInstalled {
					osExit(0, log, "Version is already installed, not attempting to install agent")
				}
			}
		}

		if reInstallAgent {
			log.Infof("Starting agent uninstallation")
			if err := helperUnInstallAgent(log, packageManager, serviceManager, ""); err != nil {
				osExit(1, log, "Failed to uninstall the agent: %v", err)
			}
			log.Infof("Agent uninstalled successfully")

			log.Infof("Starting agent installation")
			if err := helperInstallAgent(log, packageManager, serviceManager, artifactsDir); err != nil {
				osExit(1, log, "Failed to install agent: %v", err)
			}
			log.Infof("Agent installed successfully")
		} else {
			log.Infof("Agent is not installed on the system, Starting agent installation")
			if err := helperInstallAgent(log, packageManager, serviceManager, artifactsDir); err != nil {
				osExit(1, log, "Failed to install agent: %v", err)
			}
			log.Infof("Agent installed successfully")
		}
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
			if err = startAgent(serviceManager, log); err != nil {
				osExit(1, log, "Failed to start agent: %v", err)
			}
			return
		}

		log.Infof("Stopping agent before registering")
		if err = servicemanagers.StopAgent(serviceManager, log); err != nil {
			osExit(1, log, "Failed to stop agent: %v", err)
		}

		log.Infof("Registering agent")
		if err = getRegisterManager().RegisterAgent(registerInputModel); err != nil {
			osExit(1, log, "Failed to register agent: %v", err)
		}

		log.Infof("Successfully registered the agent, starting agent")
		if err = startAgent(serviceManager, log); err != nil {
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
		if err = svcMgrStopAgent(serviceManager, log); err != nil {
			osExit(1, log, "Failed to shut down agent: %v", err)
		}
	}

	log.Flush()
	log.Close()
}

func performOnpremSteps(log log.T, packageManager packagemanagers.IPackageManager, verificationManager verificationmanagers.IVerificationManager, serviceManager servicemanagers.IServiceManager) error {
	// this path will be used for storing artifacts downloaded
	ssmSetupCLIExecutablePath, err := getExecutableFolderPath()
	if err != nil {
		return fmt.Errorf("could not get the ssm-setup-cli executable path: %v", err)
	}

	// create directories for storing artifacts
	setupCLIArtifactsPath, err := fileUtilCreateTemp(ssmSetupCLIExecutablePath, utility.SSMSetupCLIArtifactsFolderName)
	if err != nil {
		return fmt.Errorf("could not create temp folder in ssm setup cli executable path: %v", err)
	}
	if err = fileUtilMakeDirs(setupCLIArtifactsPath); err != nil {
		return fmt.Errorf("could not create SSM Setup CLI directory: %v", err)
	}
	childDirectory := "child_"
	setupCLIArtifactsPath, err = fileUtilCreateTemp(setupCLIArtifactsPath, childDirectory)
	if err != nil {
		return fmt.Errorf("could not create ssm setup cli artifacts temp directory in child folder: %v", err)
	}

	isNano, err := isPlatformNano(log)
	if isNano {
		log.Infof("Windows Nano platform detected")
	}

	// Initialize download manager
	log.Infof("Initialize download manager")
	downloadManager := getDownloadManager(log, region, manifestUrl, nil, setupCLIArtifactsPath, isNano)
	if downloadManager == nil {
		return fmt.Errorf("failed to intialize download manager")
	}

	if manifestUrl != "" {
		if !strings.HasPrefix(manifestUrl, "https://") {
			return fmt.Errorf("manifest url is not https")
		}
	}

	version = strings.TrimSpace(version)
	ssmSetupCLIPath := filepath.Join(ssmSetupCLIExecutablePath, utility.SSMSetupCLIBinary)
	latestExecutableCheckSum, err := utilityCheckSum(ssmSetupCLIPath)
	if err != nil {
		return fmt.Errorf("error computing installed executable checksum: %v", err)
	}
	err = downloadManager.DownloadLatestSSMSetupCLI(setupCLIArtifactsPath, latestExecutableCheckSum)
	if err != nil {
		return fmt.Errorf("error downloading latest SSM-Setup-CLI executable: %v", err)
	}

	err = installAndVerifyAgent(log, packageManager, verificationManager, serviceManager, downloadManager, setupCLIArtifactsPath, isNano)
	if err != nil {
		return err
	}

	if register {
		err = registerOnPrem(log, packageManager, serviceManager)
		if err != nil {
			return err
		}
	}
	// sleeping for 5 seconds for the process to launch
	timeSleep(5 * time.Second)
	present, err := checkForSingleAgentProcesses(log)
	if err != nil {
		return fmt.Errorf("error while checking agent process count: %v", err)
	}
	if !present {
		return fmt.Errorf("multiple/no processes found: %v", err)
	}
	log.Infof("Agent registration completed")
	return nil
}

func installAndVerifyAgent(log log.T,
	packageManager packagemanagers.IPackageManager,
	verificationManager verificationmanagers.IVerificationManager,
	serviceManager servicemanagers.IServiceManager,
	downloadManager downloadmanager.IDownloadManager,
	setupCLIArtifactsPath string,
	isNano bool) error {
	var targetAgentVersion string
	// Check whether SSM-Setup-CLI is latest or not.
	latestVersion, err := downloadManager.GetLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to get latest version: %v", err)
	}
	if strings.EqualFold(version, utility.LatestVersionString) {
		targetAgentVersion = latestVersion
	}

	// Check whether the requested target version is installed
	var isAgentInstalled bool // This bool says that an agent is already installed
	if isAgentInstalled, err = packageManager.IsAgentInstalled(); err != nil {
		return fmt.Errorf("failed to get agent installation status: %v", err)
	}
	if !isAgentInstalled && strings.TrimSpace(version) == "" {
		version = utility.StableVersionString
	}

	// Get stable version
	var stableVersion string
	if version == utility.StableVersionString {
		stableVersion, err = downloadManager.GetStableVersion()
		if err != nil {
			return fmt.Errorf("failed to get stable version: %v", err)
		}
		targetAgentVersion = stableVersion
	}

	agentVersionInstalled, err := packageManager.GetInstalledAgentVersion()
	if isAgentInstalled && agentVersionInstalled == "" {
		return fmt.Errorf("error while getting installed agent version: %v", err)
	}
	if isAgentInstalled && targetAgentVersion == "" {
		targetAgentVersion = agentVersionInstalled
	}
	log.Infof("Installed agent version - %v", agentVersionInstalled)
	var isTargetAgentInstalled bool // This bool says that the target version is already installed
	sourceVersionFilePaths, targetVersionFilePaths := "", ""
	uninstallNeeded := false
	if isAgentInstalled {
		if strings.EqualFold(agentVersionInstalled, targetAgentVersion) {
			log.Infof("Version already installed %v: ", targetAgentVersion)
			isTargetAgentInstalled = true
		} else if versionutil.Compare(agentVersionInstalled, targetAgentVersion, true) > 0 {
			if downgrade == false {
				return fmt.Errorf("downgrade flag is not set")
			}
			sourceVersionFilePaths = filepath.Join(setupCLIArtifactsPath, agentVersionInstalled)
			if err = fileUtilMakeDirs(sourceVersionFilePaths); err != nil {
				return fmt.Errorf("could not create source version directory: %v", err)
			}
			err = downloadManager.DownloadArtifacts(agentVersionInstalled, manifestUrl, sourceVersionFilePaths)
			if err != nil {
				return fmt.Errorf("error while downloading source agent: %v", err)
			}
			uninstallNeeded = true
		}
	}

	if !isTargetAgentInstalled {
		// Download target agent version artifacts
		log.Infof("Started downloaded agent artifacts for version: %v", targetAgentVersion)
		targetVersionFilePaths = filepath.Join(setupCLIArtifactsPath, targetAgentVersion)
		if err = fileUtilMakeDirs(targetVersionFilePaths); err != nil {
			return fmt.Errorf("could not update folder permissions: %v", err)
		}
		err = downloadManager.DownloadArtifacts(targetAgentVersion, manifestUrl, targetVersionFilePaths)
		if err != nil {
			return fmt.Errorf("error while downloading agent %v", err)
		}
		log.Infof("Successfully downloaded agent artifacts for version: %v", version)

		if !skipSignatureValidation && verificationManager != nil {
			fileExtension := packageManager.GetFileExtension()

			// Download will happen only for Linux
			signaturePath, err := downloadManager.DownloadSignatureFile(targetAgentVersion, targetVersionFilePaths, fileExtension)
			if err != nil {
				return fmt.Errorf("failed to download signature file %v", err)
			}
			log.Infof("Signature path: %v", signaturePath)

			log.Infof("Start agent signature verification")
			err = verificationManager.VerifySignature(log, signaturePath, targetVersionFilePaths, fileExtension)
			if err != nil {
				return fmt.Errorf("failed to verify signature file: %v", err)
			}
			log.Infof("Agent signature verification ended successfully")
		}
	}

	if uninstallNeeded {
		err = helperUnInstallAgent(log, packageManager, serviceManager, sourceVersionFilePaths)
		if err != nil {
			return fmt.Errorf("uninstallation failed for source version: %v", err)
		}
		timeSleep(2 * time.Second)
		if isAgentInstalled, err = packageManager.IsAgentInstalled(); err != nil || isAgentInstalled {
			return fmt.Errorf("failed to get agent installation status: %v", err)
		}
	}

	log.Infof("Attempting to configure agent")
	configManager := getConfigurationManager()
	if err = configManager.CreateUpdateAgentConfigWithOnPremIdentity(); err != nil {
		return fmt.Errorf("return failed to update agent config %v", err)
	}

	if !isTargetAgentInstalled {
		log.Infof("Starting agent installation")
		if err := helperInstallAgent(log, packageManager, serviceManager, targetVersionFilePaths); err != nil {
			return fmt.Errorf("installation failed %v", err)
		}
		if isNano {
			if err = startAgent(serviceManager, log); err != nil {
				return fmt.Errorf("failed while starting agent: %v", err)
			}
		}
		log.Infof("Agent installed successfully")
	}
	return nil
}

func registerOnPrem(log log.T, packageManager packagemanagers.IPackageManager, serviceManager servicemanagers.IServiceManager) error {
	var err error
	log.Info("Verifying agent is installed before attempting to register")
	if isInstalled, err := packageManager.IsAgentInstalled(); err != nil {
		return fmt.Errorf("agent has not started still")
	} else if !isInstalled {
		return fmt.Errorf("agent is not installed")
	}
	log.Info("Verified agent is installed")

	registrationInfo := getRegistrationInfo()
	instanceId := registrationInfo.InstanceID(log, "", registration.RegVaultKey)

	if instanceId != "" {
		log.Infof("Agent already registered with instance id: %s", instanceId)
	} else {
		log.Info("Agent is not registered")
	}

	if instanceId != "" && !override {
		log.Info("skipping registration because override flag is not set, just starting agent back")
		if err = startAgent(serviceManager, log); err != nil {
			return fmt.Errorf("%v", err)
		}
	} else {
		log.Infof("Stopping agent before registering")
		if err = svcMgrStopAgent(serviceManager, log); err != nil {
			return fmt.Errorf("failed to stop agent: %v", err)
		}

		log.Infof("Registering agent")
		if err = getRegisterManager().RegisterAgent(registerInputModel); err != nil {
			return fmt.Errorf("failed to register agent: %v", err)
		}

		log.Infof("Successfully registered the agent, starting agent")
		if err = startAgent(serviceManager, log); err != nil {
			return fmt.Errorf("failed to start agent: %v", err)
		}

		log.Infof("Successfully started agent, reloading registration info")
		registrationInfo.ReloadInstanceInfo(log, "", registration.RegVaultKey)
		instanceId = registrationInfo.InstanceID(log, "", registration.RegVaultKey)
		if instanceId == "" {
			return fmt.Errorf("failed to get new instance id from registration info after registration")
		} else {
			log.Infof("Successfully registered the instance with AWS SSM using Managed instance-id: %s", instanceId)
		}
	}
	return err
}

func checkForSingleAgentProcesses(log log.T) (bool, error) {
	processExecutor := newProcessExecutor(log)
	processes, err := processExecutor.Processes()
	if err != nil {
		return false, fmt.Errorf("failure to get processes list: %v", err)
	}
	processCount := 0
	for _, processName := range processes {
		if strings.HasSuffix(strings.ToLower(processName.Executable), strings.ToLower(utility.AgentBinary)) {
			log.Infof("Process Path: %v", strings.ToLower(processName.Executable))
			processCount++
		}
	}
	log.Infof("Agent process count: %v", processCount)
	if processCount >= 2 || processCount == 0 {
		return false, fmt.Errorf("invalid agent process count. Please uninstall additional processes")
	}
	return true, nil
}

func setVerifyGreenGrassParams(log log.T) {
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
	verifyParams(log, greengrassParamVerification)
	registerInputModel = &registermanager.RegisterAgentInputModel{
		Region: region,
		Role:   role,
		Tags:   tags,
	}
}

func setVerifyOnpremParams(log log.T) {
	verifyParams(log, onPremParamVerification)
	registerInputModel = &registermanager.RegisterAgentInputModel{
		Region:         region,
		ActivationCode: activationCode,
		ActivationId:   activationId,
	}
}

func setParams() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.CommandLine.Usage = flagUsage

	// This section of flags is for both Onprem and greengrass
	flag.StringVar(&environment, "env", "", "")
	flag.StringVar(&region, "region", "", "")
	flag.BoolVar(&help, "help", false, "")

	// This section of flags is only for Onprem
	flag.BoolVar(&install, "install", false, "")
	flag.BoolVar(&shutdown, "shutdown", false, "")
	flag.StringVar(&artifactsDir, "artifacts-dir", "", "")

	// agent registration related flags
	flag.BoolVar(&register, "register", false, "")
	flag.StringVar(&activationCode, "activation-code", "", "")
	flag.StringVar(&activationId, "activation-id", "", "")
	flag.BoolVar(&override, "override", false, "")
	flag.StringVar(&role, "role", "", "")
	flag.StringVar(&tags, "tags", "", "") // only for greengrass

	// below flags only for onprem environment
	flag.StringVar(&version, "version", "", "")
	flag.StringVar(&manifestUrl, "manifest-url", "", "")
	flag.BoolVar(&downgrade, "downgrade", false, "")

	flag.BoolVar(&skipSignatureValidation, "skip-signature-validation", false, "")

	flag.Parse()
}

func hasAgentAlreadyInstalled(versionStr string) (bool, error) {
	val, err := versionutil.VersionCompare(versionStr, agentVersioning.Version)
	if err != nil {
		return false, fmt.Errorf("failed to compare with already installed agent version: %w", err)
	}

	return val == 0, nil
}

func getExecutableFolderPath() (string, error) {
	ex, err := osExecutable()
	if err != nil {
		return "", err
	}
	exPath, err := evalSymLinks(filePathDir(ex))
	if err != nil {
		return "", err
	}

	return exPath, nil
}

func verifyParams(log log.T, additionalVerifier func() string) {
	if flag.NFlag() == 0 || help {
		flagUsage()
		osExit(0, log, "")
	}

	log.Info("Setup parameters:")
	log.Infof("env=%v", environment)

	log.Infof("install=%v", install)
	log.Infof("shutdown=%v", shutdown)
	log.Infof("role=%v", role)
	log.Infof("tags=%v", tags)

	log.Infof("register=%v", register)
	log.Infof("region=%v", region)
	log.Infof("override=%v", override)

	log.Infof("version=%v", version)
	log.Infof("manifest-url=%v", manifestUrl)
	log.Infof("artifactsDir=%v", artifactsDir)
	log.Infof("skip-signature-validation=%v", skipSignatureValidation)

	var errMessage string
	errMessage += additionalVerifier()

	if region == "" {
		errMessage += "Region required. "
	}

	if errMessage != "" {
		flagUsage()
		osExit(1, log, "Invalid parameters - %v", errMessage)
	}
}

func onPremParamVerification() string {
	var errMessage string
	if !register {
		errMessage += "Action required (register). "
	}
	if activationId != "" || activationCode != "" {
		if activationCode == "" {
			errMessage += "Activation code required for on-prem registration. "
		}
		if activationId == "" {
			errMessage += "Activation id required for on-prem registration. "
		}
	} else {
		errMessage += "Activation id/code required for on-prem registration. "
	}
	return errMessage
}

func greengrassParamVerification() string {
	var errMessage string
	if artifactsDir == "" {
		errMessage += "Artifacts directory required. "
	}

	if !install && !register && !shutdown {
		errMessage += "Action required (install|register|shutdown). "
	}

	if register && role == "" {
		errMessage += "Role required for registration. "
	}
	return errMessage
}

func flagUsage() {

	fmt.Fprintln(os.Stderr, "\n-env   \tInstruct cli what environment you are installing to ('greengrass'/'onprem'). Default set to 'onprem'  \t(OPTIONAL)")

	fmt.Fprintln(os.Stderr, "\nCommand-line Usage for ONPREM environment:")
	fmt.Fprintln(os.Stderr, "\t-region        \tRegion used for ssm agent download location and registration \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t-version\tVersion of the ssm agent to download ('stable' or 'latest'). Default set to 'stable' if agent is not already installed \t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\t-downgrade\tSet when the agent needs to be downgraded \t(OPTIONAL but REQUIRED during downgrade)")
	fmt.Fprintln(os.Stderr, "\t-skip-signature-validation\tSkip signature validation \t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\t-register      \tRegister ssm agent if unregistered or override is set \t(REQUIRED)")
	fmt.Fprintln(os.Stderr, "\t\t-activation-code  \tSSM Activation Code for Onprem environment \t(REQUIRED and paired with activation-id)")
	fmt.Fprintln(os.Stderr, "\t\t-activation-id  \tSSM Activation ID for Onprem environment \t(REQUIRED and paired with Activation code)")
	fmt.Fprintln(os.Stderr, "\t\t-override \t\tOverride existing registration if present \t(OPTIONAL)")

	fmt.Fprintln(os.Stderr, "\nCommand-line Usage for GREENGRASS environment:")
	fmt.Fprintln(os.Stderr, "\t-artifacts-dir \tDirectory for ssm agent install package and install/register scripts")
	fmt.Fprintln(os.Stderr, "\t-region        \tRegion used for ssm agent download location and registration")
	fmt.Fprintln(os.Stderr, "\t-download      \tDownload ssm agent install package based on platform")
	fmt.Fprintln(os.Stderr, "\t-install       \tInstall ssm agent based on platform")
	fmt.Fprintln(os.Stderr, "\t-shutdown      \tStop SSM Agent")
	fmt.Fprintln(os.Stderr, "\t-register      \tRegister ssm agent if unregistered or override is set")
	fmt.Fprintln(os.Stderr, "\t\t-role     \t\tRole ssm agent will be registered with           \t(REQUIRED and paired with tags)")
	fmt.Fprintln(os.Stderr, "\t\t-tags     \t\tTags to attach to ssm instance on registrations  \t(OPTIONAL and paired WITH role)")
	fmt.Fprintln(os.Stderr, "\t\t-activation-code  \tSSM Activation Code for Onprem environment \t\t(REQUIRED and paired with activation-id)")
	fmt.Fprintln(os.Stderr, "\t\t-activation-id  \tSSM Activation ID for Onprem environment \t\t(REQUIRED and paired with Activation code)")
	fmt.Fprintln(os.Stderr, "\t\t-override \t\tOverride existing registration if present        \t(OPTIONAL)")
	fmt.Fprintln(os.Stderr, "\t\t-tags     \t\tTags to attach to ssm instance on registrations  \t(OPTIONAL)")

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
	loggerInstance := &logger.DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger
	formatFilter := &logger.ContextFormatFilter{Context: []string{}}
	return &logger.Wrapper{Format: formatFilter, M: LogMutex, Delegate: loggerInstance}
}

func initializeLoggerForOnprem() log.T {
	logFileName := filepath.Join(logger.DefaultLogDir, "ssm-setup-cli.log")
	// log to console
	seelogConfig := `
<seelog type="sync">
    <exceptions>
        <exception filepattern="test*" minlevel="error"/>
    </exceptions>
    <outputs formatid="fmtinfo">
        <console formatid="fmtinfo"/>
        <rollingfile type="size" filename="` + logFileName + `" maxsize="30000000" maxrolls="5"/>
    </outputs>
    <formats>
        <format id="fmterror" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtdebug" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtinfo" format="%Date %Time %LEVEL %Msg%n"/>
    </formats>
</seelog>
`
	seelogger, _ := seelog.LoggerFromConfigAsBytes([]byte(seelogConfig))
	loggerInstance := &logger.DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger
	formatFilter := &logger.ContextFormatFilter{Context: []string{}}
	return &logger.Wrapper{Format: formatFilter, M: LogMutex, Delegate: loggerInstance}
}
