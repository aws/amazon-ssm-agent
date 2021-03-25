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

// Package windowscontainerutil implements the the install and uninstall steps for windows for the configurecontainers plugin.
package windowscontainerutil

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
)

const (
	DOCKER_DOWNLOAD_URL         = "https://download.docker.com/components/engine/windows-server/cs-1.12/docker.zip"
	DOCKER_UNCOMPRESS_DIRECTORY = "C:\\Program Files"
	DOCKER_INSTALLED_DIRECTORY  = DOCKER_UNCOMPRESS_DIRECTORY + "\\docker"
)

func RunInstallCommands(context context.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var platformVersion string
	var parameters []string
	var requireReboot bool

	var isNanoServer bool
	var output string

	log := context.Log()
	platformVersion, err = dep.PlatformVersion(log)
	if err != nil {
		log.Error("Error detecting platform version", err)
		out.MarkAsFailed(fmt.Errorf("Error detecting platform version: %v", err))
		return
	}
	log.Debug("Platform Version:", platformVersion)
	if !strings.HasPrefix(platformVersion, "10") {
		out.MarkAsFailed(errors.New("ConfigureDocker is only supported on Microsoft Windows Server 2016."))
		return
	}

	isNanoServer, err = dep.IsPlatformNanoServer(log)
	if err != nil {
		log.Error("Error detecting if Nano Server", err)
		out.MarkAsFailed(fmt.Errorf("Error detecting if Nano Server: %v", err))
		return
	}
	if isNanoServer {
		command = "(Get-PackageProvider -ListAvailable).Name"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting package providers", err)
			out.MarkAsFailed(fmt.Errorf("Error getting package providers: %v", err))
			return
		}
		log.Debug("Get-PackageProvider output:", output)
		packageInstalled := strings.Contains(output, "NanoServerPackage")

		if !packageInstalled {
			out.AppendInfo("Installing Nano Server package provider.")
			command = `Install-PackageProvider -Name Nuget -MinimumVersion 2.8.5.201 -Force`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(60, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing Nuget package provider", err)
				out.MarkAsFailed(fmt.Errorf("Error installing Nuget package provider: %v", err))
				return
			}
			log.Debug("Install Package provider output:", output)

			command = `Save-Module -Path "$env:programfiles\WindowsPowerShell\Modules\" -Name NanoServerPackage -minimumVersion 1.0.1.0`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(60, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error saving module", err)
				out.MarkAsFailed(fmt.Errorf("Error saving Nano server package: %v", err))
				return
			}
			log.Debug("Save-Module output:", output)

			command = `Import-PackageProvider NanoServerPackage`
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error importing package", err)
				out.MarkAsFailed(fmt.Errorf("Error importing package: %v", err))
				return
			}
			log.Debug("Import-PackageProvider output:", output)
		}

		//Install containers package
		command = "(Get-Package -providername NanoServerPackage).Name"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting microsoft-nanoserver-containers-package", err)
			out.MarkAsFailed(fmt.Errorf("Error getting microsoft-nanoserver-containers-package: %v", err))
			return
		}
		log.Debug("Get-Package output:", output)
		packageInstalled = strings.Contains(output, "Microsoft-NanoServer-Containers-Package")

		if !packageInstalled {
			out.AppendInfo("Installing containers package.")
			command = "Install-NanoServerPackage microsoft-nanoserver-containers-package"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing microsoft-nanoserver-containers-package", err)
				out.MarkAsFailed(fmt.Errorf("Error installing microsoft-nanoserver-containers-package: %v", err))
				return
			}
			log.Debug("Install-NanoServerPackage output:", output)
			requireReboot = true
		}
	} else {
		//install windows containers feature
		command = "(Get-WindowsFeature -Name containers).Installed"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting containers feature", err)
			out.MarkAsFailed(fmt.Errorf("Error getting containers feature: %v", err))
			return
		}
		log.Debug("Get-WindowsFeature output:", output)
		packageInstalled := strings.Contains(output, "True")

		if !packageInstalled {
			out.AppendInfo("Installing Windows containers feature.")
			command = "(Install-WindowsFeature -Name containers).RestartNeeded"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(30, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error installing Windows containers feature", err)
				out.MarkAsFailed(fmt.Errorf("Error installing Windows containers feature: %v", err))
				return
			}
			log.Debug("Install-WindowsFeature output:", output)
			requireReboot = strings.Contains(output, "Yes")
		}
	}

	//Create docker config if it does not exist
	daemonConfigPath := os.Getenv("ProgramData") + "\\docker\\config\\daemon.json"
	daemonConfigContent := `
{
    "fixed-cidr": "172.17.0.0/16"
}
`

	if err := dep.SetDaemonConfig(daemonConfigPath, daemonConfigContent); err != nil {
		log.Error("Error writing Docker daemon config file", err)
		out.MarkAsFailed(fmt.Errorf("Error writing Docker daemon config file: %v", err))
		return
	}

	//Download docker
	var downloadOutput artifact.DownloadOutput
	downloadOutput, err = dep.ArtifactDownload(context, artifact.DownloadInput{SourceURL: DOCKER_DOWNLOAD_URL, DestinationDirectory: appconfig.DownloadRoot})
	if err != nil {
		log.Errorf("failed to download file from %v: %v", DOCKER_DOWNLOAD_URL, err)
		return
	}
	log.Debugf("Zip file downloaded to %v", downloadOutput.LocalFilePath)

	_, installedErr := os.Stat(DOCKER_INSTALLED_DIRECTORY)
	if downloadOutput.IsUpdated || installedErr != nil {
		out.AppendInfo("Unzipping Docker to program files directory.")
		//uncompress docker zip
		fileutil.Uncompress(log, downloadOutput.LocalFilePath, DOCKER_UNCOMPRESS_DIRECTORY)
	}

	// delete downloaded file, if it exists
	pluginutil.CleanupFile(log, downloadOutput.LocalFilePath)

	//Set this process's path environment variable to include Docker
	if !strings.Contains(strings.ToLower(os.Getenv("path")), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		out.AppendInfo("Setting process path variable to include docker directory")
		//set envvariable for this process
		os.Setenv("path", DOCKER_INSTALLED_DIRECTORY+";"+os.Getenv("path"))

	}
	log.Debug("Path set to ", os.Getenv("path"))

	//set path env variable for machine to include Docker
	var currentSystemPathValue string
	currentSystemPathValue, _, err = dep.LocalRegistryKeyGetStringValue(`System\CurrentControlSet\Control\Session Manager\Environment`, "Path")
	if err != nil {
		log.Error("Error getting current machine registry key value", err)
		out.MarkAsFailed(fmt.Errorf("Error getting current machine registry key value: %v", err))
		return
	}
	log.Debug("System Path set to ", currentSystemPathValue)
	if !strings.Contains(strings.ToLower(currentSystemPathValue), strings.ToLower(DOCKER_INSTALLED_DIRECTORY)) {
		out.AppendInfo("Setting machine path variable to include docker directory")
		command = "setx"
		parameters = []string{"-m", "path", os.Getenv("path")}
		var setPathOutput string
		setPathOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting machine path environment variable", err)
			out.MarkAsFailed(fmt.Errorf("Error setting machine path environment variable: %v", err))
			return
		}
		log.Debug("setx path output:", setPathOutput)
	}

	//reboot if needed
	if requireReboot {
		out.AppendInfo("Rebooting machine to complete install")
		log.Debug("require reboot is true")
		out.SetStatus(contracts.ResultStatusSuccessAndReboot)
		return
	}

	//Check if docker daemon registered
	var dockerServiceStatusOutput string
	command = "(Get-Service docker).Status"
	parameters = make([]string, 0)
	dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error getting Docker service status", err)
		out.MarkAsFailed(fmt.Errorf("Error getting Docker service status: %v", err))
		return
	}
	log.Debug("Get-Service output:", dockerServiceStatusOutput)

	ServiceRunning := strings.HasPrefix(dockerServiceStatusOutput, "Running")

	//Register Service
	if len(strings.TrimSpace(dockerServiceStatusOutput)) == 0 {
		out.AppendInfo("Registering dockerd.")

		command = `dockerd`
		log.Debug("dockerd cmd:", command)
		parameters = []string{"--register-service"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, DOCKER_INSTALLED_DIRECTORY, "", "", "", false)
		if err != nil {
			log.Error("Error registering Docker service", err)
			out.MarkAsFailed(fmt.Errorf("Error registering Docker service: %v", err))
			return
		}
		log.Debug("dockerd output:", dockerServiceStatusOutput)
		//set service to delayed start
		out.AppendInfo("set dockerd service configuration.")
		command = "sc.exe"
		parameters = []string{"config", "docker", "start=delayed-auto"}
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(10, log, command, parameters, "", "", "", "", false)
		if err != nil {
			log.Error("Error setting delayed start for Docker service", err)
			out.MarkAsFailed(fmt.Errorf("Error setting delayed start for Docker service: %v", err))
			return
		}
		log.Debug("sc output:", dockerServiceStatusOutput)
		//sleep 10 sec after registering
		time.Sleep(10 * time.Second)
	}
	err = dep.LocalRegistryKeySetDWordValue(`SYSTEM\CurrentControlSet\services\docker`, "AutoStartDelay", 240)
	if err != nil {
		log.Error("Error opening registry key to set Docker delayed start", err)
		out.MarkAsFailed(fmt.Errorf("Error opening registry key to set Docker delayed start: %v", err))
		return
	}

	//Start service
	if !ServiceRunning {
		out.AppendInfo("Starting Docker service.")
		command = "Start-Service docker"
		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(300, log, command, parameters, "", "", "", "", true)
		if err != nil {

			log.Error("Error starting Docker service", err)
			out.MarkAsFailed(fmt.Errorf("Error starting Docker service: %v", err))
			return
		}
		log.Debug("start-service output:", dockerServiceStatusOutput)
	}
	out.AppendInfo("Installation complete")
	log.Debug("require reboot:", requireReboot)
	out.SetStatus(contracts.ResultStatusSuccess)
	return
}

func RunUninstallCommands(log log.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var parameters []string
	var requireReboot bool
	var platformVersion string

	var isNanoServer bool
	var output string

	platformVersion, err = dep.PlatformVersion(log)
	if err != nil {
		log.Error("Error detecting platform version", err)
		out.MarkAsFailed(fmt.Errorf("Error detecting platform version: %v", err))
		return
	}
	log.Debug("Platform Version:", platformVersion)
	if !strings.HasPrefix(platformVersion, "10") {
		out.MarkAsFailed(errors.New("ConfigureDocker is only supported on Microsoft Windows Server 2016."))
		return
	}

	//Check if docker daemon registered and running
	var dockerServiceStatusOutput string
	command = "(Get-Service docker).Status"
	parameters = make([]string, 0)
	dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", true)
	if err != nil {
		log.Error("Error getting Docker service status", err)
		out.MarkAsFailed(fmt.Errorf("Error getting Docker service status: %v", err))
		return
	}
	log.Debug("Get-Service output:", dockerServiceStatusOutput)

	ServiceRunning := strings.Contains(dockerServiceStatusOutput, "Running")

	//Stop service
	if ServiceRunning {
		out.AppendInfo("Stopping Docker Service.")
		command = "Stop-Service docker"
		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(180, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error stopping Docker service", err)
			out.MarkAsFailed(fmt.Errorf("Error stopping Docker service: %v", err))
			return
		}
		log.Debug("stop-service output:", dockerServiceStatusOutput)
	}

	//Unregister Service
	if len(strings.TrimSpace(dockerServiceStatusOutput)) > 0 {
		out.AppendInfo("Unregistering dockerd service.")
		command = "(Get-WmiObject -Class Win32_Service -Filter \"Name='docker'\").delete()"

		parameters = make([]string, 0)
		dockerServiceStatusOutput, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, DOCKER_INSTALLED_DIRECTORY, "", "", "", true)
		if err != nil {
			log.Error("Error unregistering Docker service", err)
			out.MarkAsFailed(fmt.Errorf("Error unregistering Docker service: %v", err))
			return
		}
		log.Debug("dockerd output:", dockerServiceStatusOutput)

	}

	//Remove docker directory
	if _, err := os.Stat(DOCKER_INSTALLED_DIRECTORY); err == nil {
		out.AppendInfo("Removing Docker directory.")
		os.RemoveAll(DOCKER_INSTALLED_DIRECTORY)
	}

	//check if Nano
	isNanoServer, err = dep.IsPlatformNanoServer(log)
	if err != nil {
		log.Error("Error detecting if Nano Server", err)
		out.MarkAsFailed(fmt.Errorf("Error detecting if Nano Server: %v", err))
		return
	}

	if isNanoServer {
		out.AppendInfo("Removing packages from Nano server not currently supported.")
	} else {
		//uninstall windows containers feature
		command = "(Get-WindowsFeature -Name containers).Installed"
		parameters = make([]string, 0)
		output, err = dep.UpdateUtilExeCommandOutput(50, log, command, parameters, "", "", "", "", true)
		if err != nil {
			log.Error("Error getting containers feature", err)
			out.MarkAsFailed(fmt.Errorf("Error getting containers feature: %v", err))
			return
		}
		log.Debug("Get-WindowsFeature output:", output)
		packageInstalled := strings.Contains(output, "True")

		if packageInstalled {
			out.AppendInfo("Uninstalling containers Windows feature.")
			command = "(Uninstall-WindowsFeature -Name containers).RestartNeeded"
			parameters = make([]string, 0)
			output, err = dep.UpdateUtilExeCommandOutput(300, log, command, parameters, "", "", "", "", true)
			if err != nil {
				log.Error("Error uninstalling containers Windows feature", err)
				out.MarkAsFailed(fmt.Errorf("Error uninstalling containers Windows feature: %v", err))
				return
			}
			log.Debug("Uninstall-WindowsFeature output:", output)
			requireReboot = strings.Contains(output, "Yes")
			log.Debug("Requireboot:", requireReboot)
		}
		//reboot if needed
		if requireReboot {
			out.AppendInfo("Rebooting machine to complete install")
			log.Debug("require reboot is true", requireReboot)
			out.SetStatus(contracts.ResultStatusSuccessAndReboot)
			return
		}
	}
	out.AppendInfo("Uninstallation complete.")
	log.Debug("Uninstallation complete")
	out.SetStatus(contracts.ResultStatusSuccess)

	return
}
