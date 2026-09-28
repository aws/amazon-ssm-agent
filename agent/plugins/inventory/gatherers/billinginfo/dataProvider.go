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

// Package billinginfo contains a billing info gatherer.
package billinginfo

import (
	"context"
	"strings"

	"github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"

	agentcontext "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

var isOnPremInstance = identity.IsOnPremInstance

// decouples for easy testability
var queryIdentityDocument = queryInstanceIdentityDocument

// CollectBillingInfoData collects billing information for linux
func CollectBillingInfoData(context agentcontext.T) (data []model.BillingInfoData) {

	log := context.Log()

	log.Infof("Getting %v data", GathererName)

	if isOnPremInstance(context.Identity()) {
		log.Infof("Do not call Billing info, On-Premises instance")
		return
	}

	identityDocument, err := queryIdentityDocument()
	if err != nil {
		log.Errorf("GetInstanceIdentityDocument failed with error: %v", err.Error())
		return
	}
	log.Infof("Instance identity document %v", identityDocument)

	data = parseInstanceIdentityDocumentOutput(context, identityDocument)
	log.Infof("Parsed BillingInfo output data %v", data)
	return data

}

// Collects relevant fields (marketplaceProductCodes, billingProducts) from GetInstanceIdentityDocument output.
// Here is a sample GetInstanceIdentityDocument output (some lines omitted):
// {
// "marketplaceProductCodes" : [],
// "version" : "2017-09-30",
// "instanceType" : "t2.micro",
// "billingProducts" : [ "bp-878787", "bp-23478" ]
// }
func queryInstanceIdentityDocument() (imds.InstanceIdentityDocument, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		// IMDS ignores RetryMaxAttempts and only honors a Retryer func, so set
		// the retry bound via WithRetryer instead of WithRetryMaxAttempts.
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = 3
			})
		}),
	)
	if err != nil {
		return imds.InstanceIdentityDocument{}, err
	}

	ec2MetadataService := imds.NewFromConfig(cfg)
	result, err := ec2MetadataService.GetInstanceIdentityDocument(context.TODO(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return imds.InstanceIdentityDocument{}, err
	}
	return result.InstanceIdentityDocument, nil
}

func parseInstanceIdentityDocumentOutput(context agentcontext.T, identityDocument imds.InstanceIdentityDocument) (data []model.BillingInfoData) {
	log := context.Log()

	billingProductIds := identityDocument.BillingProducts
	marketPlaceProductIds := identityDocument.MarketplaceProductCodes
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
		itemContent := model.BillingInfoData{
			BillingProductId: billingProductId,
		}
		data = append(data, itemContent)
	}
	return data
}
