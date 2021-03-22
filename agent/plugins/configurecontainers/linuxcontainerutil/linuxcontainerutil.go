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

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
)

func RunInstallCommands(context context.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var info updateinfo.T
	log := context.Log()
	info, err = dep.GetInstanceInfo(context)
	if err != nil {
		log.Error("Error determining Linux variant", err)
		out.MarkAsFailed(fmt.Errorf("Error determining Linux variant: %v", err))
		return
	}
	if info.GetPlatform() == updateconstants.PlatformUbuntu {
		log.Error("Ubuntu platform is not currently supported", err)
		out.MarkAsFailed(fmt.Errorf("Ubuntu platform is not currently supported: %v", err))
		return
	} else if info.GetPlatform() == updateconstants.PlatformLinux {
		runAmazonLinuxPlatformInstallCommands(log, orchestrationDirectory, out)
		return
	} else if info.GetPlatform() == updateconstants.PlatformRedHat {
		runRedhatLinuxPlatformInstallCommands(log, orchestrationDirectory, out)
		return
	} else {
		log.Error("Unsupported Linux variant", err)
		out.MarkAsFailed(fmt.Errorf("Unsupported Linux variant: %v", err))
		return
	}
}

func runAmazonLinuxPlatformInstallCommands(log log.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var output string
	var parameters []string

	out.AppendInfo("Updating yum")
	command = "yum"
	parameters = []string{"update", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum update", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum update: %v", err))
		return
	}
	log.Debug("yum update:", output)

	out.AppendInfo("Installation docker through yum")
	command = "yum"
	parameters = []string{"install", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum install: %v", err))
		return
	}
	log.Debug("yum install:", output)

	out.AppendInfo("Starting docker service")
	command = "service"
	parameters = []string{"docker", "start"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running service docker start", err)
		out.MarkAsFailed(fmt.Errorf("Error running ervice docker start: %v", err))
		return
	}
	log.Debug("Service docker start:", output)

	out.AppendInfo("Installation complete")
	out.MarkAsSucceeded()
	return
}

func runRedhatLinuxPlatformInstallCommands(log log.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var output string
	var parameters []string

	out.AppendInfo("Installing yum-utils")
	command = "yum"
	parameters = []string{"install", "-y", "yum-utils"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum install: %v", err))
		return
	}
	log.Debug("yum install:", output)

	out.AppendInfo("Add docker repo")
	command = "yum-config-manager"
	parameters = []string{"--add-repo", "https://docs.docker.com/engine/installation/linux/repo_files/centos/docker.repo"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum-config-manage", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum-config-manager: %v", err))
		return
	}
	log.Debug("yum-config-manager:", output)

	out.AppendInfo("Update yum package index")
	command = "yum"
	parameters = []string{"makecache", "fast"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum makecache", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum makecache: %v", err))
		return
	}
	log.Debug("yum makecache:", output)

	out.AppendInfo("Installation docker through yum")
	command = "yum"
	parameters = []string{"install", "-y", "docker-engine"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum install", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum install: %v", err))
		return
	}
	log.Debug("yum install:", output)

	out.AppendInfo("Starting docker service")
	command = "systemctl"
	parameters = []string{"start", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running systemctl docker start", err)
		out.MarkAsFailed(fmt.Errorf("Error running systemctl docker start: %v", err))
		return
	}
	log.Debug("systemctl docker start:", output)

	out.AppendInfo("Installation complete")
	out.MarkAsSucceeded()
	return
}

func RunUninstallCommands(context context.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var info updateinfo.T
	log := context.Log()
	info, err = dep.GetInstanceInfo(context)
	if err != nil {
		log.Error("Error determining Linux variant", err)
		out.MarkAsFailed(fmt.Errorf("Error determining Linux variant: %v", err))
		return
	}
	if info.GetPlatform() == updateconstants.PlatformUbuntu {
		log.Error("Ubuntu platform is not currently supported", err)
		out.MarkAsFailed(fmt.Errorf("Ubuntu platform is not currently supported: %v", err))
		return
	} else if info.GetPlatform() == updateconstants.PlatformLinux {
		runAmazonLinuxPlatformUninstallCommands(log, orchestrationDirectory, out)
		return
	} else if info.GetPlatform() == updateconstants.PlatformRedHat {
		runRedhatLinuxPlatformUninstallCommands(log, orchestrationDirectory, out)
		return
	} else {
		log.Error("Unsupported Linux variant", err)
		out.MarkAsFailed(fmt.Errorf("Unsupported Linux variant: %v", err))
		return
	}
}

func runAmazonLinuxPlatformUninstallCommands(log log.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var output string
	var parameters []string
	out.AppendInfo("Removing docker though yum")
	command = "yum"
	parameters = []string{"remove", "-y", "docker"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum remove", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum remove: %v", err))
		return
	}
	log.Debug("yum remove:", output)
	out.AppendInfo("Uninstall complete")
	out.MarkAsSucceeded()
	return
}

func runRedhatLinuxPlatformUninstallCommands(log log.T, orchestrationDirectory string, out iohandler.IOHandler) {
	var err error
	var command string
	var output string
	var parameters []string
	out.AppendInfo("Removing docker though yum")
	command = "yum"
	parameters = []string{"remove", "-y", "docker-engine"}
	output, err = dep.UpdateUtilExeCommandOutput(120, log, command, parameters, "", "", "", "", false)
	if err != nil {
		log.Error("Error running yum remove", err)
		out.MarkAsFailed(fmt.Errorf("Error running yum remove: %v", err))
		return
	}
	log.Debug("yum remove:", output)
	out.AppendInfo("Uninstall complete")
	out.MarkAsSucceeded()
	return
}
