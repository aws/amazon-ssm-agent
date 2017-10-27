// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package ssmdocresource

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// dependency on SSM to get document
type ssmdeps interface {
	GetDocument(log log.T, docName string, docVersion string) (response *ssm.GetDocumentOutput, err error)
}

type ssmDocDepImpl struct {
	ssmSvc ssmsvc.Service
}

func (s ssmDocDepImpl) GetDocument(log log.T, docName string, docVersion string) (response *ssm.GetDocumentOutput, err error) {
	return s.ssmSvc.GetDocument(log, docName, docVersion)
}
