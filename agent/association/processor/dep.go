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

// Package processor manage polling of associations, dispatching association to processor
package processor

import (
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

var assocSvc assocService = assocSvcImp{}

type assocService interface {
	ListAssociations(log log.T, ssmSvc ssm.Service, instanceID string) (model.AssociationDetail, error)
}

type assocSvcImp struct{}

func (assocSvcImp) ListAssociations(log log.T, ssmSvc ssm.Service, instanceID string) (model.AssociationDetail, error) {
	return service.ListAssociations(log, ssmSvc, instanceID)
}
