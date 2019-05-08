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

// +build darwin freebsd linux netbsd openbsd

// Package billinginfo contains a billing info gatherer.
package billinginfo

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	/*
	 * Command to get the instance meta-data.
	 * More details about instance-meta data is at
	 * https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
	 */
	curlCmd                = "curl"
	curlCmdArgs            = "http://169.254.169.254/latest/dynamic/instance-identity/document"
	billingProductsKey     = "\"billingProducts\" :"
	marketPlaceProductsKey = "\"marketplaceProductCodes\" :"
)

// cmdExecutor decouples exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// CollectBillingInfoData collects billing information for linux
func CollectBillingInfoData(context context.T) (data []model.BillingInfoData) {

	log := context.Log()

	var output []byte
	var err error
	cmd := curlCmd
	args := []string{curlCmdArgs}

	log.Infof("Executing command: %v %v", cmd, args)
	if output, err = cmdExecutor(cmd, args...); err != nil {
		log.Errorf("Failed to execute command : %v %v; error: %v", cmd, args, err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		return
	}

	log.Infof("Parsing output %v", string(output))
	r := parseCurlOutput(context, string(output))
	log.Infof("Parsed BillingInfo output %v", r)
	return r
}

// parseCurlOutput collects relevant fields (marketplaceProductCodes, billingProducts) from curl output.
// Here is a sample curl output (some lines omitted):
// {
// "marketplaceProductCodes" : null,
// "version" : "2017-09-30",
// "instanceType" : "t2.micro",
// "billingProducts" : [ "bp-878787", "bp-23478" ]
// }
func parseCurlOutput(context context.T, output string) (data []model.BillingInfoData) {
	log := context.Log()

	// get license included billing product ids.
	billingProductIds := getAllBillingProductIds(context, output, billingProductsKey)
	// get market place billing product ids.
	marketPlaceProductIds := getAllBillingProductIds(context, output, marketPlaceProductsKey)

	// concatenate the two results
	billingProductIds = append(billingProductIds, marketPlaceProductIds...)

	/*
		We parse the BillingProduct from the instance meta-data and once we have that info we need
		to create a json object in the below formats.
		[
			{
				"BillingProductId" : "bp-6ba54002"
			}
		]
		or
		[
			{
				"BillingProductId" : "bp-6ba54002"
			},
			{
				"BillingProductId" : "bp-6ba54003"
			}
		]
	*/
	if billingProductIds == nil {
		// Nothing to report for AWS:BillingInfo
		log.Infof("Nothing to report for gatherer %v", GathererName)
		return
	}
	for _, billingProductId := range billingProductIds {
		// trim quotes and spaces
		billingProductId = strings.TrimSpace(billingProductId)
		billingProductId = billingProductId[1 : len(billingProductId)-1]

		itemContent := model.BillingInfoData{
			BillingProductId: strings.TrimSpace(billingProductId),
		}
		data = append(data, itemContent)
	}
	return
}

func getAllBillingProductIds(context context.T, output string, key string) (bp []string) {
	// Gets the row of entry for the billingProduct curl entry
	bpRow := getFieldValue(output, key)
	// convert the billing product row into array of billing products
	keyStartPos := strings.Index(bpRow, "[")

	// If the billingProduct is null
	if keyStartPos < 0 {
		fmt.Println("Null string")
		return
	}
	keyEndPos := strings.Index(bpRow, "]")

	// split the billingproduct ids by "," delimiter.
	bp = strings.Split(bpRow[keyStartPos+1:keyEndPos], ",")
	return
}

// getFieldValue looks for the first substring of the form "key: value \n" and returns the "value"
// if no such field found, returns empty string
func getFieldValue(input string, key string) string {
	keyStartPos := strings.Index(input, key)
	if keyStartPos < 0 {
		return ""
	}

	// add "\n" sentinel in case the key:value pair is on the last line and there is no newline at the end
	afterKey := input[keyStartPos+len(key)+1:] + "\n"
	valueEndPos := strings.Index(afterKey, "\n")
	return strings.TrimSpace(afterKey[:valueEndPos])
}
