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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"github.com/aws/amazon-ssm-agent/agent/longrunning/datastore"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
)

// dataStoreT defines the operations that manager uses to interact with its data-store
type dataStoreT interface {
	Write(data map[string]plugin.PluginInfo) error
	Read() (map[string]plugin.PluginInfo, error)
}

// ds contains the implementation of long running plugin manager's dataStore
type ds struct {
	dsImpl datastore.FsStore
}

// Write writes new data in the data-store
func (d ds) Write(data map[string]plugin.PluginInfo) error {
	return d.dsImpl.Write(data)
}

// Read reads data from the data-store
func (d ds) Read() (map[string]plugin.PluginInfo, error) {
	return d.dsImpl.Read()
}

var dataStore dataStoreT = ds{
	dsImpl: datastore.FsStore{},
}
