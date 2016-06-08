// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// package registration provides managed instance information
package registration

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/vault/fsvault"
)

// dependency for fileutil
var file fileT = fileUtility{}

type fileUtility struct{}

type fileT interface {
	WriteAllText(filePath string, text string) (err error)
}

// WriteAllText writes all text content to the specified file
func (fileUtility) WriteAllText(filePath string, text string) (err error) {
	return fileutil.WriteAllText(filePath, text)
}

// dependency for vault
var vault iiVault = &iiFsVault{}

type iiVault interface {
	Retrieve(key string) (data []byte, err error)
	Store(key string, data []byte) (err error)
}

type iiFsVault struct{}

func (iiFsVault) Retrieve(key string) ([]byte, error) { return fsvault.Retrieve(key) }
func (iiFsVault) Store(key string, data []byte) error { return fsvault.Store(key, data) }
