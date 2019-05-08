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

// +build windows

// Package billinginfo contains a billinginfo gatherer.
package billinginfo

import (
	"encoding/json"
	"os/exec"
	"strings"

	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	cmd = "powershell"
	/*
	 * Command to get the instance meta-data and get the product codes entry from it.
	 * We are getting the billing products from LicenseIncluded and Marketplace instances.
	 * More details about instance-meta data is at
	 * https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	 */
	cmdArgsToGetBillingInfo = `Invoke-RestMethod -uri  http://169.254.169.254/latest/dynamic/instance-identity/document |  foreach {$_.billingProducts, $_.marketplaceProductCodes} | foreach { $_ } | foreach  { if ($_ -ne $null ) {  @{"BillingProductId"=$_} } } | ConvertTo-Json`
)

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// Gets the billingProducts info from the instance meta-data.
func CollectBillingInfoData(context context.T) (data []model.BillingInfoData) {
	var output []byte
	var err error
	log := context.Log()
	log.Infof("Getting %v data", GathererName)

	log.Infof("Collecting LicenseIncluded and Marketplace product codes by executing command:\n%v %v", cmd, cmdArgsToGetBillingInfo)

	if output, err = cmdExecutor(cmd, cmdArgsToGetBillingInfo); err == nil {
		cmdOutput := string(output)
		cmdOutput = convertToJsonArray(cmdOutput)

		if len(cmdOutput) == 0 || cmdOutput == "[]" {
			log.Infof("Nothing to report for gatherer %v", GathererName)
			return
		}
		log.Infof("Command output: %v", cmdOutput)
		if err = json.Unmarshal([]byte(cmdOutput), &data); err != nil {
			err = fmt.Errorf("Unable to parse command output - %v", err.Error())
			log.Error(err.Error())
			log.Infof("Error parsing command output - no data to return")
		}
	} else {
		log.Infof("Failed to execute command : %v %v with error - %v",
			cmd,
			cmdArgsToGetBillingInfo,
			err.Error())
		log.Errorf("Command failed with error: %v", string(output))
	}

	return
}

// convertToJsonArray does the two below things
// 1. If the billingProductId is null then it returns an empty string.
// 2. cmdArgsToGetBillingInfo uses ConvertTo-Json which generates an array if there are multiple product codes
// but if there is only single product code then ConvertTo-Json generates an object. This method checks if the
// billingInfoEntries is a object in which case it converts it into an array by adding '[' ']' around the object.
func convertToJsonArray(billingInfoEntries string) (str string) {
	// Check if billing Product Id is null
	keyStartPos := strings.Index(billingInfoEntries, "null")
	if keyStartPos >= 0 {
		// billingProductId is null
		return
	}

	//trim spaces
	str = strings.TrimSpace(billingInfoEntries)

	// Check if there is "[" in the string.
	// If "[" is not present then we add
	// "[" in beginning & "]" at the end to create valid json string
	keyStartPos = strings.Index(str, "[")
	if keyStartPos < 0 {
		str = fmt.Sprintf("[%v]", str)
	}

	return str
}
