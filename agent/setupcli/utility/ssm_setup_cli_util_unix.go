//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

package utility

import (
	"fmt"
	"os"
	"os/user"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	SSMSetupCLIBinary = "ssm-setup-cli"

	// ExpectedServiceRunningUser is the user we expect the agent to be running as
	ExpectedServiceRunningUser = "root"

	// AgentBinary is the name of agent binary
	AgentBinary = appconfig.DefaultAgentName
)

// IsRunningElevatedPermissions checks if the ssm-setup-cli is being executed as administrator
func IsRunningElevatedPermissions() error {
	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	if currentUser.Username == ExpectedServiceRunningUser {
		return nil
	} else {
		return fmt.Errorf("ssm-setup-cli needs to be executed by %s", ExpectedServiceRunningUser)
	}
}

// HasRootPermissions shows whether the folder path has root permission
func HasRootPermissions(folderPath string) (bool, error) {
	fileInfo, err := os.Stat(folderPath)
	if err != nil {
		return false, err
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("unable to get syscall.Stat_t for folder")
	}

	// Check if the owner of the folder is root (UID 0)
	if stat.Uid == 0 {
		return true, nil
	}

	return false, nil
}
