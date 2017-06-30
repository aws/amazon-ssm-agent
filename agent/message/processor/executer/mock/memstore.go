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

// Package executer provides interfaces as document execution logic
package executermocks

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// Initialize the in-memory map
var documentMap = make(map[string]*model.DocumentState)
var logger = log.NewMockLog()

//DocumentMemStore is used at integration test to store document state object in memory to peel off tedious file IOs
type DocumentMemoryStore struct {
	context    context.T
	documentID string
	state      model.DocumentState
}

func NewMemStore(context context.T, docID string) DocumentMemoryStore {

	return DocumentMemoryStore{
		context:    context,
		documentID: docID,
	}
}

func (m DocumentMemoryStore) Save() {
	//copy the current state value to the memory map
	var newState = m.state
	documentMap[m.documentID] = &newState
	return
}

func (m DocumentMemoryStore) Load() *model.DocumentState {
	if val, ok := documentMap[m.documentID]; ok {
		m.state = *val
		return &m.state
	}
	logger.Errorf("document %v is not found in the memcache", m.documentID)
	return nil
}
