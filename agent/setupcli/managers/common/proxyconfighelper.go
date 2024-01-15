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

package common

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
)

// SetProxyConfig sets the proxy config for the agent
func SetProxyConfig(log log.T) {
	log.Info("Setting proxy config")
	proxyConfig := proxyconfig.SetProxyConfig(log)
	log.Info("Proxy environment variables:")
	for key, value := range proxyConfig {
		log.Infof(key + ": " + value)
	}
}
