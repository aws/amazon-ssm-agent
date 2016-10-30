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
//
// +build windows
//
// Package

package configurecontainers

import (
	"io/ioutil"
	"os"
	"strings"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"golang.org/x/sys/windows/registry"

)

const (
	DOCKER_DOWNLOAD_URL         = "https://download.docker.com/components/engine/windows-server/cs-1.12/docker.zip"
	DOCKER_UNCOMPRESS_DIRECTORY = "C:\\Program Files"
	DOCKER_INSTALLED_DIRECTORY  = DOCKER_UNCOMPRESS_DIRECTORY + "\\docker"
)

func runInstallCommands(log log.T, pluginInput ConfigureContainerPluginInput, orchestrationDirectory string) (out ConfigureContainerPluginOutput) {
	var err error
	var command string
	var parameters []string
	var requireReboot bool
	//util := updateutil.Utility{CustomUpdateExecutionTimeoutInSeconds: 3600}
	var isNanoServer bool
	isNanoServer, err = platform.IsPlatformNanoServer(log)
	if err != nil {
		log.Error("Error detecting if Nano Server", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	if isNanoServer {
		var output string
		command = "Get-PackageProvider -name NanoServerPackage"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting package provider", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("Get-PackageProvider output:", output)
		packageInstalled := strings.Contains(output, "NanoServerPackage")

		if !packageInstalled {
			command = `Save-Module -Path "$env:programfiles\WindowsPowerShell\Modules\" -Name NanoServerPackage -minimumVersion 1.0.1.0`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error saving module", err)
				out.Errors = append(out.Errors, err.Error())
				return
			}
			log.Info("Save-Module output:", output)

			command = `Import-PackageProvider NanoServerPackage`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error importing package", err)
				out.Errors = append(out.Errors, err.Error())
				return
			}
			log.Info("Import-PackageProvider output:", output)
		}

		//Install containers package
		command = "Get-Package -providername NanoServerPackage -Name microsoft-nanoserver-containers-package"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting microsoft-nanoserver-containers-package", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("Get-Package output:", output)
		packageInstalled = strings.Contains(output, "Microsoft-NanoServer")

		if !packageInstalled {
			command = "Install-NanoServerPackage microsoft-nanoserver-containers-package"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing microsoft-nanoserver-containers-package", err)
				out.Errors = append(out.Errors, err.Error())
				return
			}
			log.Info("Install-NanoServerPackage output:", output)
		}
	} else {
		//install windows containers feature

		var installFeatureOutput string
		command = "(Install-WindowsFeature -Name containers).RestartNeeded"
		parameters = make([]string, 0)
		installFeatureOutput, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error installing containers Windows feature", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("Install-WindowsFeature output:", installFeatureOutput)
		requireReboot = strings.HasPrefix(installFeatureOutput, "Yes")
	}

	//Create docker config if it does not exist
	daemonConfigPath := os.Getenv("ProgramData") + "\\docker\\config\\daemon.json"
	daemonConfigContent := `
{
    "fixed-cidr": "172.17.0.0/16"
}
`
	if _, err := os.Stat(daemonConfigPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(daemonConfigPath), 744)
		err := ioutil.WriteFile(daemonConfigPath, []byte(daemonConfigContent), 0644)
		if err != nil {
			log.Error("Error writing docker daemon config file", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
	}

	//Download docker
	var downloadOutput artifact.DownloadOutput
	downloadOutput, err = dep.ArtifaceDownload(log, artifact.DownloadInput{SourceURL: DOCKER_DOWNLOAD_URL, DestinationDirectory: os.TempDir()})
	if downloadOutput.IsUpdated {
		//uncompress docker zip
		fileutil.Uncompress(downloadOutput.LocalFilePath, DOCKER_UNCOMPRESS_DIRECTORY)
	}
	log.Info("downloaded to ", downloadOutput.LocalFilePath)

	//Set this process's path environment variable to include Docker
	if !strings.Contains(strings.ToLower(os.Getenv("path")), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		//set envvariable for this process
		os.Setenv("path", DOCKER_INSTALLED_DIRECTORY+";"+os.Getenv("path"))

	}
	log.Info("Path set to ", os.Getenv("path"))

	//set path env variable for machine tp include Docker
	var regKey registry.Key
	regKey, err = dep.RegistryOpenKey(registry.LOCAL_MACHINE, `System\CurrentControlSet\Control\Session Manager\Environment`, registry.ALL_ACCESS)
	if err != nil {
		log.Error("Error getting current machine registry key", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	defer regKey.Close()
	var currentSystemPathValue string
	currentSystemPathValue, _, err = dep.RegistryKeyGetStringValue(regKey, "Path")
	if err != nil {
		log.Error("Error getting current machine registry key value", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	log.Info("System Path set to ", currentSystemPathValue)
	if !strings.Contains(strings.ToLower(currentSystemPathValue), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		command = "setx"
		parameters = []string{"-m", "path", os.Getenv("path")}
		log.Info("setx path command:", command)
		var setPathOutput string
		setPathOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting machine path environment variable", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("setx path output:", setPathOutput)
	}

	//reboot if needed
	if requireReboot {
		out.Status = contracts.ResultStatusSuccessAndReboot
	}
	log.Info("require reboot", requireReboot)

	//Check if docker daemon registered
	var dockerServiceStatusOutput string
	command = "(Get-Service docker).Status"
	parameters = make([]string, 0)
	dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error getting docker service status", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	log.Info("Get-Service output:", dockerServiceStatusOutput)

	ServiceRunning := strings.HasPrefix(dockerServiceStatusOutput, "Running")

	//Register Service
	if len(strings.TrimSpace(dockerServiceStatusOutput)) == 0 {
		log.Info("dockerd installed directory:", DOCKER_INSTALLED_DIRECTORY)
		command = "dockerd"
		parameters = []string{"--register-service"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, DOCKER_INSTALLED_DIRECTORY, "", "", "", false)
		if err != nil {
			log.Error("Error starting docker service", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("dockerd output:", dockerServiceStatusOutput)
		//set service to delayed start
		command = "sc.exe"
		parameters = []string{"config", "docker", "start=delayed-auto"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting delayed start for docker service", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("sc output:", dockerServiceStatusOutput)
	}
	//set delayed start time in registry
	var created bool
	regKey, err = dep.RegistryOpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\services\docker`, registry.ALL_ACCESS)
	if err != nil {
		log.Error("Error creating registry key to set docker delayed start", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}
	log.Info("created reg key:", created)
	defer regKey.Close()
	err = dep.RegistryKeySetDWordValue(regKey, "AutoStartDelay", 240)
	if err != nil {
		log.Error("Error opening registry key to set docker delayed start", err)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	//Start service
	if !ServiceRunning {
		command = "Start-Service docker"
		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(180, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error starting docker service", err)
			out.Errors = append(out.Errors, err.Error())
			return
		}
		log.Info("start-service output:", dockerServiceStatusOutput)
	}

	return out
}
