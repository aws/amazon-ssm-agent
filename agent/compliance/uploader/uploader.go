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

package compliance

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/compliance/model"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmSvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	stopPolicyErrorThreshold      = 10
	associationComplianceType     = "Association"
	Name                          = "ComplianceUploader"
	AssociationComplianceItemName = "AssociationComplianceItem"
)

var (
	lock sync.RWMutex
)

// T represents interface for compliance
type T interface {
	CreateNewServiceIfUnHealthy(log log.T)
	UpdateAssociationCompliance(associationId string, instanceId string, documentName string, documentVersion string, associationStatus string, executionTime time.Time) error
}

// ComplianceService wraps the Ssm Service
type ComplianceUploader struct {
	ssmSvc     ssmSvc.Service
	stopPolicy *sdkutil.StopPolicy
	name       string
	context    context.T
	optimizer  datauploader.Optimizer
}

// NewComplianceService returns a new compliance service
func NewComplianceUploader(context context.T) *ComplianceUploader {
	var err error

	ssmService := ssmSvc.NewService()
	policy := sdkutil.NewStopPolicy(Name, stopPolicyErrorThreshold)
	uploader := &ComplianceUploader{
		ssmSvc:     ssmService,
		stopPolicy: policy,
		context:    context,
		name:       Name,
	}

	if uploader.optimizer, err = datauploader.NewOptimizerImplWithLocation(
		uploader.context.Log(),
		appconfig.ComplianceRootDirName,
		appconfig.ComplianceContentHashFileName); err != nil {
		uploader.context.Log().Errorf("Unable to load optimizer for compliance service because - %v", err.Error())
		return uploader
	}

	return uploader
}

func (u *ComplianceUploader) CreateNewServiceIfUnHealthy(log log.T) {
	if u.stopPolicy == nil {
		log.Debugf("Creating new stop-policy.")
		u.stopPolicy = sdkutil.NewStopPolicy(u.name, stopPolicyErrorThreshold)
	}

	log.Debugf("Compliance service's stoppolicy before polling is %v", u.stopPolicy)
	if !u.stopPolicy.IsHealthy() {
		log.Errorf("Compliance service stopped temporarily due to internal failure. We will retry automatically")

		// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
		u.stopPolicy.ResetErrorCount()
		u.ssmSvc = ssmSvc.NewService()
	}
}

func calculateCheckSum(data []byte) (checkSum string) {
	sum := md5.Sum(data)
	checkSum = base64.StdEncoding.EncodeToString(sum[:])
	return
}

/**
 * Update association compliance status, it only report status back when status is either SUCCESS / FAILED / TIMEDOUT
 */
func (u *ComplianceUploader) UpdateAssociationCompliance(associationID string, instanceID string, documentName string, documentVersion string, associationStatus string, executionTime time.Time) error {
	if contracts.AssociationStatusTimedOut != associationStatus &&
		contracts.AssociationStatusSuccess != associationStatus &&
		contracts.AssociationStatusFailed != associationStatus {
		return nil
	}

	log := u.context.Log()

	model.UpdateAssociationComplianceItem(associationID, documentName, documentVersion, associationStatus, executionTime)
	var associationComplianceEntries = model.GetAssociationComplianceEntries()

	oldHash := u.optimizer.GetContentHash(AssociationComplianceItemName)
	newComplianceItems, itemContentHash, err := u.ConvertToSsmAssociationComplianceItems(log, associationComplianceEntries, oldHash)

	// 1. When call PutComplianceItem failed, it will fail silently  with an error message the agent should have permission to call
	// 2. When old date arrive at server side before new date, the server side will discard and use the new date
	response, err := u.ssmSvc.PutComplianceItems(
		log,
		&executionTime,
		"",
		"",
		instanceID,
		associationComplianceType,
		itemContentHash,
		newComplianceItems)

	if err != nil {
		err = fmt.Errorf("Unable to update association compliance %v", err)
		return err
	}

	if itemContentHash != oldHash {
		u.optimizer.UpdateContentHash(AssociationComplianceItemName, itemContentHash)
	}

	log.Debugf("Put compliance item %v return response %v", newComplianceItems, response)
	return nil
}

// ConvertToSsmAssociationComplianceItems converts given array of complianceItem into an array of *ssm.ComplianceItemEntry. It returns 2 such arrays - one is optimized array
// which contains only contentHash for those compliance types where the dataset hasn't changed from previous collection. The other array is non-optimized array
// which contains both contentHash & content. This is done to avoid iterating over the compliance data twice. It throws error when it encounters error during
// conversion process.
func (u *ComplianceUploader) ConvertToSsmAssociationComplianceItems(log log.T, associationComplianceEntries []*model.AssociationComplianceItem, oldHash string) (
	associationComplianceItems []*ssm.ComplianceItemEntry, contentHash string, err error) {

	log.Debugf("Transforming collected compliance data to expected format")

	var dataB []byte

	newHash := ""

	if dataB, err = json.Marshal(associationComplianceEntries); err != nil {
		return
	}

	newHash = calculateCheckSum(dataB)

	log.Debugf("Association compliance item being converted with data - %v with checksum - %v", string(dataB), newHash)

	if newHash == oldHash {
		log.Debugf("Compliance data for %v is same as before - we can just send content hash", AssociationComplianceItemName)
		return []*ssm.ComplianceItemEntry{}, newHash, nil

	}
	log.Debugf("Compliance data for %v is NOT same as before - we send the whole content", AssociationComplianceItemName)

	for _, item := range associationComplianceEntries {
		var complianceItem = &ssm.ComplianceItemEntry{
			Id:       aws.String(item.AssociationId),
			Status:   aws.String(item.ComplianceStatus),
			Severity: aws.String(item.ComplianceSeverity),
			Title:    aws.String(item.Title),
			Details: map[string]*string{
				"DocumentName":    aws.String(item.DocumentName),
				"DocumentVersion": aws.String(item.DocumentVersion),
			},
		}
		associationComplianceItems = append(associationComplianceItems, complianceItem)
	}
	return associationComplianceItems, newHash, nil

}
