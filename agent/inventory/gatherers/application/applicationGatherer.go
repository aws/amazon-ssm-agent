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

// Package application contains a dummy gatherer.

package application

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

//TODO: add unit tests.

const (
	Name                       = "AWS:Application"
	SchemaVersionOfApplication = "1.0"
)

type T struct{}

func Gatherer(context context.T) (*T, error) {
	return new(T), nil
}

func (t *T) Name() string {
	return Name
}

func (t *T) Run(context context.T, configuration inventory.Config) (inventory.Item, error) {

	//NOTE: Since this is a fake application gatherer, it is generating fake data

	var result inventory.Item
	var err error

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	result = inventory.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfApplication,
		Content:       CreateFakeApplicationData(),
		//capture time must be in UTC so that formatting to RFC3339 complies with regex at SSM
		CaptureTime: captureTime,
	}

	return result, err
}

func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}

func CreateFakeApplicationData() []inventory.ApplicationData {
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
