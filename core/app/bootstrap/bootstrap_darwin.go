// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build darwin
// +build darwin

package bootstrap

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/aws/amazon-ssm-agent/common/message"
)

func (bs *Bootstrap) createIPCFolder() error {
	return bs.createIfNotExist(message.DefaultCoreAgentChannel)
}

func (bs *Bootstrap) updateSSMUserShellProperties(logger log.T) {
	var ssmSessionUtil utility.SessionUtil
	if ok, _ := ssmSessionUtil.DoesUserExist(appconfig.DefaultRunAsUserName); ok {
		if err := ssmSessionUtil.ChangeUserShell(); err != nil {
			logger.Warnf("UserShell Update Failed: %v", err)
			return
		}
		logger.Infof("Successfully updated ssm-user shell properties")
	}
}
