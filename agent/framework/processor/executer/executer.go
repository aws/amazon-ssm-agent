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
package executer

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Executer accepts an DocumentStore object, save when necessary for crash-recovery, and return when finshes the run, while
// the caller will pick up the result from the same docStore object
type Executer interface {
	//TODO in future, docState should be de-composed into static/dynamic && plugin/document informations
	// Given a document and run it, receiving results from status update channel, return a map of plugin results
	Run(cancelFlag task.CancelFlag, //Inbound message
		docStore DocumentStore) chan contracts.DocumentResult
}

// DocumentStore is an wrapper over the document state class that provides additional persisting functions for the Executer
type DocumentStore interface {
	Save(contracts.DocumentState)
	Load() contracts.DocumentState
}

// DocumentFileStore dependent on the current file functions in docmanager to provide file save/load operations
// TODO need to refactor global lock in docmanager, or discard the entire package and impl the file IO here
type DocumentFileStore struct {
	state             contracts.DocumentState
	documentID        string
	location          string
	documentMgr       docmanager.DocumentMgr
	isLocalSaveNeeded bool
}

func NewDocumentFileStore(docID, location string, state *contracts.DocumentState, docMgr docmanager.DocumentMgr, isLocalSaveNeeded bool) DocumentFileStore {
	return DocumentFileStore{
		documentID:        docID,
		location:          location,
		state:             *state,
		documentMgr:       docMgr,
		isLocalSaveNeeded: isLocalSaveNeeded,
	}
}

// Save the document info struct to the current folder, Save() is desired only for crash-recovery
func (f *DocumentFileStore) Save(docState contracts.DocumentState) {
	//copy the state struct
	f.state = docState
	if f.isLocalSaveNeeded {
		f.documentMgr.PersistDocumentState(
			f.documentID,
			f.location,
			docState)
	}
	return
}

// Load should happen in memory
func (f *DocumentFileStore) Load() contracts.DocumentState {
	return f.state
}
