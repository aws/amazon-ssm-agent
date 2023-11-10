//go:build windows
// +build windows

package utility

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	SSMSetupCLIBinary = "ssm-setup-cli.exe"

	// AgentBinary is the name of agent binary
	AgentBinary = appconfig.DefaultAgentName + ".exe"
)

var powershellArgs = []string{"-InputFormat", "None", "-Noninteractive", "-NoProfile", "-ExecutionPolicy", "unrestricted"}

// IsRunningElevatedPermissions checks if the ssm-setup-cli is being executed as administrator
func IsRunningElevatedPermissions() error {
	checkAdminCmd := `([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] 'Administrator')`
	output, err := executePowershellCommandWithTimeout(2*time.Second, checkAdminCmd)

	if err != nil {
		return fmt.Errorf("failed to check permissions: %s", err)
	}

	if output == "True" {
		return nil
	} else if output == "False" {
		return fmt.Errorf("ssm-setup-cli needs to be executed by administrator")
	} else {
		return fmt.Errorf("unexpected permission check output: %s", output)
	}
}

func executePowershellCommandWithTimeout(timeout time.Duration, command string) (string, error) {
	args := append(powershellArgs, "-Command", command)
	return executeCommandWithTimeout(timeout, appconfig.PowerShellPluginCommandName, args...)
}

func executeCommandWithTimeout(timeout time.Duration, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	byteArr, err := exec.CommandContext(ctx, cmd, args...).Output()
	output := strings.TrimSpace(string(byteArr))

	return output, err
}

// HasRootPermissions shows whether the folder path has root permission
// For windows, this function is will always return true as Greengrass support is not available for windows still
func HasRootPermissions(folderPath string) (bool, error) {
	return true, nil
}
