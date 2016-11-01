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
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
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

	var isNanoServer bool
	var output string

	isNanoServer, err = platform.IsPlatformNanoServer(log)
	if err != nil {
		log.Error("Error detecting if Nano Server", err)
		out.MarkAsFailed(log, err)
		return out
	}
	if isNanoServer {
		command = "Get-PackageProvider -name NanoServerPackage"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting package provider", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("Get-PackageProvider output:", output)
		packageInstalled := strings.Contains(output, "NanoServerPackage")

		if !packageInstalled {
			out.Stdout += "Installing Nano Server package provider\n"
			command = `Save-Module -Path "$env:programfiles\WindowsPowerShell\Modules\" -Name NanoServerPackage -minimumVersion 1.0.1.0`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error saving module", err)
				out.MarkAsFailed(log, err)
				return out
			}
			log.Info("Save-Module output:", output)

			command = `Import-PackageProvider NanoServerPackage`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error importing package", err)
				out.MarkAsFailed(log, err)
				return out
			}
			log.Info("Import-PackageProvider output:", output)
		}

		//Install containers package
		command = "Get-Package -providername NanoServerPackage -Name microsoft-nanoserver-containers-package"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting microsoft-nanoserver-containers-package", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("Get-Package output:", output)
		packageInstalled = strings.Contains(output, "Microsoft-NanoServer")

		if !packageInstalled {
			out.Stdout += "Installing containers package\n"
			command = "Install-NanoServerPackage microsoft-nanoserver-containers-package"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing microsoft-nanoserver-containers-package", err)
				out.MarkAsFailed(log, err)
				return out
			}
			log.Info("Install-NanoServerPackage output:", output)
		}
	} else {
		//install windows containers feature

		command = "Get-WindowsFeature -Name containers"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting containers feature", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("Get-WindowsFeature output:", output)
		packageInstalled := strings.Contains(output, "Installed")

		if !packageInstalled {
			out.Stdout += "Installing containers Windows feature\n"
			command = "(Install-WindowsFeature -Name containers).RestartNeeded"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing containers Windows feature", err)
				out.MarkAsFailed(log, err)
				return out
			}
			log.Info("Install-WindowsFeature output:", output)
			requireReboot = strings.Contains(output, "Yes")
			log.Info("Requireboot:", requireReboot)
		}
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
			out.MarkAsFailed(log, err)
			return out
		}
	}

	//Download docker
	var downloadOutput artifact.DownloadOutput
	downloadOutput, err = dep.ArtifactDownload(log, artifact.DownloadInput{SourceURL: DOCKER_DOWNLOAD_URL, DestinationDirectory: os.TempDir()})
	if downloadOutput.IsUpdated {
		out.Stdout += "Unzipping Docker to program files directory\n"
		//uncompress docker zip
		fileutil.Uncompress(downloadOutput.LocalFilePath, DOCKER_UNCOMPRESS_DIRECTORY)
	}
	log.Info("downloaded to ", downloadOutput.LocalFilePath)

	//Set this process's path environment variable to include Docker
	if !strings.Contains(strings.ToLower(os.Getenv("path")), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		out.Stdout += "Setting process path variable to include docker directory\n"
		//set envvariable for this process
		os.Setenv("path", DOCKER_INSTALLED_DIRECTORY+";"+os.Getenv("path"))

	}
	log.Info("Path set to ", os.Getenv("path"))

	//set path env variable for machine to include Docker
	var regKey registry.Key
	regKey, err = dep.RegistryOpenKey(registry.LOCAL_MACHINE, `System\CurrentControlSet\Control\Session Manager\Environment`, registry.ALL_ACCESS)
	if err != nil {
		log.Error("Error getting current machine registry key", err)
		out.MarkAsFailed(log, err)
		return out
	}
	defer regKey.Close()
	var currentSystemPathValue string
	currentSystemPathValue, _, err = dep.RegistryKeyGetStringValue(regKey, "Path")
	if err != nil {
		log.Error("Error getting current machine registry key value", err)
		out.MarkAsFailed(log, err)
		return out
	}
	log.Info("System Path set to ", currentSystemPathValue)
	if !strings.Contains(strings.ToLower(currentSystemPathValue), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		out.Stdout += "Setting machine path variable to include docker directory\n"
		command = "setx"
		parameters = []string{"-m", "path", os.Getenv("path")}
		var setPathOutput string
		setPathOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting machine path environment variable", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("setx path output:", setPathOutput)
	}

	//reboot if needed
	if requireReboot {
		out.Stdout += "Rebooting machine to complete install\n"
		log.Info("require reboot is true", requireReboot)
		out.Status = contracts.ResultStatusSuccessAndReboot
		return out
	}
	log.Info("require reboot", requireReboot)

	//Check if docker daemon registered
	var dockerServiceStatusOutput string
	command = "(Get-Service docker).Status"
	parameters = make([]string, 0)
	dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error getting docker service status", err)
		out.MarkAsFailed(log, err)
		return out
	}
	log.Info("Get-Service output:", dockerServiceStatusOutput)

	ServiceRunning := strings.HasPrefix(dockerServiceStatusOutput, "Running")

	//Register Service
	if len(strings.TrimSpace(dockerServiceStatusOutput)) == 0 {
		out.Stdout += "Registering dockerd.\n"
		log.Info("dockerd installed directory:", DOCKER_INSTALLED_DIRECTORY)
		command = "dockerd"
		parameters = []string{"--register-service"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, DOCKER_INSTALLED_DIRECTORY, "", "", "", false)
		if err != nil {
			log.Error("Error registering docker service", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("dockerd output:", dockerServiceStatusOutput)
		//set service to delayed start
		out.Stdout += "set dockerd service configuration.\n"
		command = "sc.exe"
		parameters = []string{"config", "docker", "start=delayed-auto"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting delayed start for docker service", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("sc output:", dockerServiceStatusOutput)
	}
	//set delayed start time in registry
	var created bool
	regKey, err = dep.RegistryOpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\services\docker`, registry.ALL_ACCESS)
	if err != nil {
		log.Error("Error creating registry key to set docker delayed start", err)
		out.MarkAsFailed(log, err)
		return out
	}
	log.Info("created reg key:", created)
	defer regKey.Close()
	err = dep.RegistryKeySetDWordValue(regKey, "AutoStartDelay", 240)
	if err != nil {
		log.Error("Error opening registry key to set docker delayed start", err)
		out.MarkAsFailed(log, err)
		return out
	}

	//Start service
	if !ServiceRunning {
		out.Stdout += "Starting Docker service\n"
		command = "Start-Service docker"
		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(180, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error starting docker service", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("start-service output:", dockerServiceStatusOutput)
	}
	out.Stdout += "Installation complete\n"
	log.Info("require reboot is true", requireReboot)
	out.Status = contracts.ResultStatusSuccess
	return out
}

func runUninstallCommands(log log.T, pluginInput ConfigureContainerPluginInput, orchestrationDirectory string) (out ConfigureContainerPluginOutput) {
	var err error
	var command string
	var parameters []string
	var requireReboot bool

	var isNanoServer bool
	var output string

	//Check if docker daemon registered and running
	var dockerServiceStatusOutput string
	command = "(Get-Service docker).Status"
	parameters = make([]string, 0)
	dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error getting docker service status", err)
		out.MarkAsFailed(log, err)
		return out
	}
	log.Info("Get-Service output:", dockerServiceStatusOutput)

	ServiceRunning := strings.HasPrefix(dockerServiceStatusOutput, "Running")

	//Stop service
	if ServiceRunning {
		out.Stdout += "Stopping Docker Service.\n"
		command = "Stop-Service docker"
		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(180, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error stopping docker service", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("stop-service output:", dockerServiceStatusOutput)
	}

	//Unregister Service
	if len(strings.TrimSpace(dockerServiceStatusOutput)) > 0 {
		out.Stdout += "Unregistering dockerd.\n"
		log.Info("dockerd installed directory:", DOCKER_INSTALLED_DIRECTORY)
		command = "dockerd"
		parameters = []string{"--unregister-service"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, DOCKER_INSTALLED_DIRECTORY, "", "", "", false)
		if err != nil {
			log.Error("Error unregistering docker service", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("dockerd output:", dockerServiceStatusOutput)

	}

	//Remove docker directory
	if _, err := os.Stat(DOCKER_INSTALLED_DIRECTORY); err == nil {
		out.Stdout += "Removing Docker directory.\n"
		os.RemoveAll(DOCKER_INSTALLED_DIRECTORY)
	}

	//check if Nano
	isNanoServer, err = platform.IsPlatformNanoServer(log)
	if err != nil {
		log.Error("Error detecting if Nano Server", err)
		out.MarkAsFailed(log, err)
		return out
	}

	if isNanoServer {
		out.Stdout += "Removing packages from Nano server not supported.\n"

	} else {
		//uninstall windows containers feature

		command = "Get-WindowsFeature -Name containers"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting containers feature", err)
			out.MarkAsFailed(log, err)
			return out
		}
		log.Info("Get-WindowsFeature output:", output)
		packageInstalled := strings.Contains(output, "containers")

		if !packageInstalled {
			out.Stdout += "Uninstalling containers Windows feature\n"
			command = "(Uninstall-WindowsFeature -Name containers).RestartNeeded"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error uninstalling containers Windows feature", err)
				out.MarkAsFailed(log, err)
				return out
			}
			log.Info("Uninstall-WindowsFeature output:", output)
			requireReboot = strings.Contains(output, "Yes")
			log.Info("Requireboot:", requireReboot)
		}
		//reboot if needed
		if requireReboot {
			out.Stdout += "Rebooting machine to complete install\n"
			log.Info("require reboot is true", requireReboot)
			out.Status = contracts.ResultStatusSuccessAndReboot
			return out
		}
	}
	out.Stdout += "Installation complete\n"
	log.Info("require reboot is true", requireReboot)
	out.Status = contracts.ResultStatusSuccess

	return out
}
