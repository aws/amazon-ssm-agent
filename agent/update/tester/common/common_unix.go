//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package common

import (
	"github.com/aws/amazon-ssm-agent/common/message"
)

func CreateIPCChannelIfNotExists() error {
	return createIfNotExist(message.DefaultCoreAgentChannel)
}
