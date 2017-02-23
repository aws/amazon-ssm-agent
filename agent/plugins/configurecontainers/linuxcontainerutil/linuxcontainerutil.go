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
package linuxcontainerutil

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

func RunInstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var context *updateutil.InstanceContext
	context, err = dep.GetInstanceContext(log)
	if err != nil {
		log.Error("Error determining Linux variant", err)
		out.MarkAsFailed(log, fmt.Errorf("Error determining Linux variant: %v", err))
		return out
	}
	if context.Platform == updateutil.PlatformUbuntu {
		log.Error("Ubuntu platform is not currently supported", err)
		out.MarkAsFailed(log, fmt.Errorf("Ubuntu platform is not currently supported: %v", err))
		return out
	} else if context.Platform == updateutil.PlatformLinux {
		return runAmazonLinuxPlatformInstallCommands(log, orchestrationDirectory)
	} else if context.Platform == updateutil.PlatformRedHat {
		return runRedhatLinuxPlatformInstallCommands(log, orchestrationDirectory)
	} else {
		log.Error("Unsupported Linux variant", err)
		out.MarkAsFailed(log, fmt.Errorf("Unsupported Linux variant: %v", err))
		return out
	}
}

func runAmazonLinuxPlatformInstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var command string
	var output string
	var parameters []string

	out.AppendInfo(log, "Updating yum")
	command = "yum"
	parameters = []string{"update", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum update", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum update: %v", err))
		return out
	}
	log.Debug("yum update:", output)

	out.AppendInfo(log, "Installation docker through yum")
	command = "yum"
	parameters = []string{"install", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum install: %v", err))
		return out
	}
	log.Debug("yum install:", output)

	out.AppendInfo(log, "Starting docker service")
	command = "service"
	parameters = []string{"docker", "start"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running service docker start", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running ervice docker start: %v", err))
		return out
	}
	log.Debug("Service docker start:", output)

	out.AppendInfo(log, "Installation complete")
	out.MarkAsSucceeded()
	return out
}

func runRedhatLinuxPlatformInstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var command string
	var output string
	var parameters []string

	out.AppendInfo(log, "Installing yum-utils")
	command = "yum"
	parameters = []string{"install", "-y", "yum-utils"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum install: %v", err))
		return out
	}
	log.Debug("yum install:", output)

	out.AppendInfo(log, "Add docker repo")
	command = "yum-config-manager"
	parameters = []string{"--add-repo", "https://docs.docker.com/engine/installation/linux/repo_files/centos/docker.repo"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum-config-manage", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum-config-manager: %v", err))
		return out
	}
	log.Debug("yum-config-manager:", output)

	out.AppendInfo(log, "Update yum package index")
	command = "yum"
	parameters = []string{"makecache", "fast"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum makecache", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum makecache: %v", err))
		return out
	}
	log.Debug("yum makecache:", output)

	out.AppendInfo(log, "Installation docker through yum")
	command = "yum"
	parameters = []string{"install", "-y", "docker-engine"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum install: %v", err))
		return out
	}
	log.Debug("yum install:", output)

	out.AppendInfo(log, "Starting docker service")
	command = "systemctl"
	parameters = []string{"start", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running systemctl docker start", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running systemctl docker start: %v", err))
		return out
	}
	log.Debug("systemctl docker start:", output)

	out.AppendInfo(log, "Installation complete")
	out.MarkAsSucceeded()
	return out
}

func RunUninstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var context *updateutil.InstanceContext
	context, err = dep.GetInstanceContext(log)
	if err != nil {
		log.Error("Error determining Linux variant", err)
		out.MarkAsFailed(log, fmt.Errorf("Error determining Linux variant: %v", err))
		return out
	}
	if context.Platform == updateutil.PlatformUbuntu {
		log.Error("Ubuntu platform is not currently supported", err)
		out.MarkAsFailed(log, fmt.Errorf("Ubuntu platform is not currently supported: %v", err))
		return out
	} else if context.Platform == updateutil.PlatformLinux {
		return runAmazonLinuxPlatformUninstallCommands(log, orchestrationDirectory)
	} else if context.Platform == updateutil.PlatformRedHat {
		return runRedhatLinuxPlatformUninstallCommands(log, orchestrationDirectory)
	} else {
		log.Error("Unsupported Linux variant", err)
		out.MarkAsFailed(log, fmt.Errorf("Unsupported Linux variant: %v", err))
		return out
	}
}

func runAmazonLinuxPlatformUninstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var command string
	var output string
	var parameters []string
	out.AppendInfo(log, "Removing docker though yum")
	command = "yum"
	parameters = []string{"remove", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum remove", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum remove: %v", err))
		return out
	}
	log.Debug("yum remove:", output)
	out.AppendInfo(log, "Uninstall complete")
	out.MarkAsSucceeded()
	return out
}

func runRedhatLinuxPlatformUninstallCommands(log log.T, orchestrationDirectory string) (out contracts.PluginOutput) {
	var err error
	var command string
	var output string
	var parameters []string
	out.AppendInfo(log, "Removing docker though yum")
	command = "yum"
	parameters = []string{"remove", "-y", "docker-engine"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum remove", err)
		out.MarkAsFailed(log, fmt.Errorf("Error running yum remove: %v", err))
		return out
	}
	log.Debug("yum remove:", output)
	out.AppendInfo(log, "Uninstall complete")
	out.MarkAsSucceeded()
	return out
}
