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
//
// package fingerprint contains functions that helps identify an instance
package fingerprint

import "github.com/aws/amazon-ssm-agent/agent/vault/fsvault"

// dependency for vault
var vault fpVault = &fpFsVault{}

type fpVault interface {
	Retrieve(key string) (data []byte, err error)
	Store(key string, data []byte) (err error)
}

type fpFsVault struct{}

func (fpFsVault) Retrieve(key string) ([]byte, error) { return fsvault.Retrieve(key) }
func (fpFsVault) Store(key string, data []byte) error { return fsvault.Store(key, data) }
