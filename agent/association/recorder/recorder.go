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

// Package recorder records the association name of the last executed association to avoid duplicate execution
package recorder

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
)

// AssociatedDocumentName represents file recording the name of the last associated document
const AssociatedDocumentName = "InstanceDocument.json"

// AssociatedDocument contains the association name
type AssociatedDocument struct {
	AssociationID string
}

var lock sync.RWMutex

// HasExecuted returns if given document has been executed
func HasExecuted(instanceID string, associationName string) bool {
	lock.Lock()
	defer lock.Unlock()

	fileName := getFileName(instanceID)
	if !fileutil.Exists(fileName) {
		return false
	}

	var assoDoc AssociatedDocument
	if err := jsonutil.UnmarshalFile(fileName, &assoDoc); err != nil {
		return false
	}

	return assoDoc.AssociationID == associationName
}

// UpdateAssociatedDocument persist last executed association name
func UpdateAssociatedDocument(InstanceID string, associationName string) error {
	lock.Lock()
	defer lock.Unlock()
	var err error
	var content string

	fileName := getFileName(InstanceID)
	location := getLocation(InstanceID)

	//verify if parent folder exist
	if !fileutil.Exists(location) {
		if err = fileutil.MakeDirs(location); err != nil {
			return fmt.Errorf("cannot make directory of %v because: %v", location, err)
		}
	}

	associatedDoc := AssociatedDocument{}
	associatedDoc.AssociationID = associationName
	if content, err = jsonutil.Marshal(associatedDoc); err != nil {
		return err
	}

	//it's fine even if we overwrite the content of previous file
	if _, err = fileutil.WriteIntoFileWithPermissions(
		fileName,
		content,
		os.FileMode(int(appconfig.ReadWriteAccess))); err != nil {
		return err
	}
	return nil
}

// getLocation returns the full path for recording last associated document.
func getLocation(instanceID string) string {
	return path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfAssociation)
}

// getFileName returns the full file name of the last associated document.
func getFileName(instanceID string) string {
	return path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfAssociation,
		AssociatedDocumentName)
}
