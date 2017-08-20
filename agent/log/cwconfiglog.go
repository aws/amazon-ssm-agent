// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package ssmlog is used to initialize ssm functional logger
package log

import (
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
)

// CWJson represents the data structure of the cloudwatch configuration file
// which contains essential information required to configure cloudwatch
type cWJson struct {
	EngineConfig cWEngineConfig `json:"EngineConfiguration"`
}

// CWEngineConfig represents the data structure for the EngineConfiguration
// field in the cloudwatch configuration file
type cWEngineConfig struct {
	Components   []interface{} `json:"Components"`
	PollInterval string        `json:"PollInterval"`
	Flows        cWFlows       `json:"Flows"`
}

// CWFlows represents the flows field in the cloudwatch configuration file
type cWFlows struct {
	Flow []interface{} `json:"Flows"`
}

// PrintCWConfig checks if the config file has credentials and returns a string
// with the credentials removed, for printing
func PrintCWConfig(jsonConfig string, log T) string {
	var config cWJson

	if err := jsonutil.Unmarshal(jsonConfig, &config); err != nil {
		log.Error("Unmarshalling CW config file failed - ", err)
	}

	// grabbing the components field in the configuration that could have exposed credentials
	components := config.EngineConfig.Components

	for iter, comps := range components {
		if cwComponents, ok := comps.(map[string]interface{}); ok {
			if cwParameters, ok := cwComponents["Parameters"]; ok {
				var parameters map[string]interface{}
				var ok bool
				if parameters, ok = cwParameters.(map[string]interface{}); ok {
					scrubCreds(parameters)
				}

				cwComponents["Parameters"] = cwParameters
			}
		}
		components[iter] = comps
	}
	config.EngineConfig.Components = components
	var err error
	var unexposed_string string
	if unexposed_string, err = jsonutil.MarshalIndent(config); err != nil {
		log.Error("Marshalling CW config file failed - ", err)
	}
	return unexposed_string
}

func scrubCreds(config map[string]interface{}) {
	if _, ok := config["AccessKey"]; ok {
		config["AccessKey"] = ""
	}

	if _, ok := config["SecretKey"]; ok {
		config["SecretKey"] = ""
	}

}
