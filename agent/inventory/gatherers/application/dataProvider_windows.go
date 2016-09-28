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

// Package application contains application gatherer.

// +build windows

package application

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

// TODO: add unit tests

// CollectApplicationData collects application data for windows platform
func CollectApplicationData(context context.T) []inventory.ApplicationData {
	//implementation is missing
	return CreateMultipleApplicationEntries()
}

// CreateMultipleApplicationEntries generates fake data with 2 entries for aws:application data
func CreateMultipleApplicationEntries() []inventory.ApplicationData {
	var data []inventory.ApplicationData

	//visual studio
	visualStudio := inventory.ApplicationData{
		Name:          "Visual Studio 10",
		Publisher:     "Microsoft Corporation",
		Version:       "14.1.0.1985",
		InstalledTime: "2016-01-01T09:10:10Z",
	}

	//adobe reader 10
	adobeReader := inventory.ApplicationData{
		Name:          "Adobe Reader 10",
		Publisher:     "Adobe",
		Version:       "10.10.1.3",
		InstalledTime: "2016-01-01T09:10:10Z",
	}

	data = append(data, visualStudio)
	data = append(data, adobeReader)

	return data
}

// CreateSingleApplicationEntry generates fake data with 1 entry for aws:application data
func CreateSingleApplicationEntry() inventory.ApplicationData {
	//visual studio
	visualStudio := inventory.ApplicationData{
		Name:          "Visual Studio 10",
		Publisher:     "Microsoft Corporation",
		Version:       "14.1.0.1985",
		InstalledTime: "2016-01-01T09:10:10Z",
	}

	return visualStudio
}
