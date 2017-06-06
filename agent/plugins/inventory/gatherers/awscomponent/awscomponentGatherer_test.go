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

// Package awscomponent contains a aws component gatherer.
package awscomponent

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

const (
	ubuntuOSName   = "Ubuntu"
	amzLinuxOSName = "Amazon Linux Ami"
	windowsOSName  = "Windows"
)

// sample data used for testing

var sampleAppDataForAmazonLinux = []model.ApplicationData{
	{
		Name:            "pcre",
		URL:             "http://www.pcre.org/",
		Publisher:       "Amazon.com",
		Version:         "8.21",
		Architecture:    "x86_64",
		InstalledTime:   "1461974300",
		ApplicationType: "System Environment/libraries",
	},
	{
		Name:            "kbd-misc",
		URL:             "http://ftp.altlinux.org/pub/people/legion/kbd",
		Publisher:       "Amazon.com",
		Version:         "1.5",
		Architecture:    "noarch",
		InstalledTime:   "1461974292",
		ApplicationType: "System Environment Base",
	},
	{
		Name:            "amazon-ssm-agent",
		URL:             "http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html",
		Publisher:       "Amazon.com",
		Version:         "1.2.0.0",
		Architecture:    "x86_64",
		InstalledTime:   "1475774764",
		ApplicationType: "Amazon/Tools",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "aws-apitools-mon",
		URL:             "http://aws.amazon.com/cloudwatch",
		Publisher:       "Amazon.com",
		Version:         "1.0.20.0",
		Architecture:    "noarch",
		InstalledTime:   "1475774764",
		ApplicationType: "Amazon/Tools",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "aws-amitools-ec2",
		URL:             "http://aws.amazon.com/ec2",
		Publisher:       "Amazon AWS",
		Version:         "1.5.7",
		Architecture:    "x86_64",
		InstalledTime:   "1461974328",
		ApplicationType: "System Environment/Base",
		CompType:        model.AWSComponent,
	},
}

var sampleAWSComponentDataForAmazonLinux = []model.ApplicationData{
	{
		Name:            "amazon-ssm-agent",
		URL:             "http://docs.aws.amazon.com/ssm/latest/APIReference/Welcome.html",
		Publisher:       "Amazon.com",
		Version:         "1.2.0.0",
		Architecture:    "x86_64",
		InstalledTime:   "1475774764",
		ApplicationType: "Amazon/Tools",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "aws-apitools-mon",
		URL:             "http://aws.amazon.com/cloudwatch",
		Publisher:       "Amazon.com",
		Version:         "1.0.20.0",
		Architecture:    "noarch",
		InstalledTime:   "1475774764",
		ApplicationType: "Amazon/Tools",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "aws-amitools-ec2",
		URL:             "http://aws.amazon.com/ec2",
		Publisher:       "Amazon AWS",
		Version:         "1.5.7",
		Architecture:    "x86_64",
		InstalledTime:   "1461974328",
		ApplicationType: "System Environment/Base",
		CompType:        model.AWSComponent,
	},
}

var sampleAppDataForLinuxOtherThanAmazonLinux = []model.ApplicationData{
	{
		Name:            "accountsservice",
		URL:             "http://cgit.freedesktop.org/accountsservice/",
		Publisher:       "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>",
		Version:         "0.6.35-0ubuntu7.2",
		Architecture:    "amd64",
		InstalledTime:   "",
		ApplicationType: "admin",
	},
	{
		Name:            "amazon-ssm-agent",
		URL:             "",
		Publisher:       "Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>",
		Version:         "1.2.0.0-1",
		Architecture:    "amd64",
		InstalledTime:   "",
		ApplicationType: "admin",
		CompType:        model.AWSComponent,
	},
}

var sampleAWSComponentDataForLinuxOtherThanAmazonLinux = []model.ApplicationData{
	{
		Name:            "amazon-ssm-agent",
		URL:             "",
		Publisher:       "Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>",
		Version:         "1.2.0.0-1",
		Architecture:    "amd64",
		InstalledTime:   "",
		ApplicationType: "admin",
		CompType:        model.AWSComponent,
	},
}

var sampleAppDataForWindows = []model.ApplicationData{
	{
		Name:            "Notepad++",
		URL:             "",
		Publisher:       "Notepad++ Team",
		Version:         "6.9.2",
		Architecture:    "64-Bit",
		InstalledTime:   "",
		ApplicationType: "",
	},
	{
		Name:            "Google Update Helper",
		URL:             "",
		Publisher:       "Google Inc.",
		Version:         "1.3.31.5",
		Architecture:    "64-Bit",
		InstalledTime:   "20161012",
		ApplicationType: "",
	},
	{
		Name:            "AWS Tools for Windows",
		URL:             "",
		Publisher:       "Amazon Web Services Developer Relations",
		Version:         "3.9.344.0",
		Architecture:    "64-Bit",
		InstalledTime:   "20160512",
		ApplicationType: "",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "AWS PV Drivers",
		URL:             "",
		Publisher:       "Amazon Web Services",
		Version:         "7.3.2",
		Architecture:    "32-Bit",
		InstalledTime:   "20150813",
		ApplicationType: "",
		CompType:        model.AWSComponent,
	},
}

var sampleAWSComponentDataForWindows = []model.ApplicationData{
	{
		Name:            "AWS Tools for Windows",
		URL:             "",
		Publisher:       "Amazon Web Services Developer Relations",
		Version:         "3.9.344.0",
		Architecture:    "64-Bit",
		InstalledTime:   "20160512",
		ApplicationType: "",
		CompType:        model.AWSComponent,
	},
	{
		Name:            "AWS PV Drivers",
		URL:             "",
		Publisher:       "Amazon Web Services",
		Version:         "7.3.2",
		Architecture:    "32-Bit",
		InstalledTime:   "20150813",
		ApplicationType: "",
		CompType:        model.AWSComponent,
	},
}

// Mock implementation to provide sample data used for testing

func MockGetApplicationDataForAmazonLinux(context context.T) []model.ApplicationData {
	return sampleAppDataForAmazonLinux
}

func MockGetApplicationDataForLinuxOSOtherThanAmazonLinux(context context.T) []model.ApplicationData {
	return sampleAppDataForLinuxOtherThanAmazonLinux
}

func MockGetApplicationDataForWindows(context context.T) []model.ApplicationData {
	return sampleAppDataForWindows
}

// Mock implementations to provide different OS for testing

func MockPlatformInfoProviderReturningAmazonLinux(log log.T) (name string, err error) {
	return amzLinuxOSName, nil
}

func MockPlatformInfoProviderReturningUbuntu(log log.T) (name string, err error) {
	return ubuntuOSName, nil
}

func MockPlatformInfoProviderReturningWindows(log log.T) (name string, err error) {
	return windowsOSName, nil
}

func MockPlatformInfoProviderReturningError(log log.T) (name string, err error) {
	return "", fmt.Errorf("Random error")
}

func TestRun(t *testing.T) {

	var data []model.Item

	//setup
	c := context.NewMockDefault()
	g := Gatherer(c)
	getApplicationData = MockGetApplicationDataForAmazonLinux

	data, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "Unexpected error thrown")
	assert.Equal(t, 1, len(data), "AWSComponent gatherer always returns 1 inventory type data - which is why number of entries must be 1.")
}

func TestCollectApplicationData(t *testing.T) {

	var data []model.ApplicationData

	//setup
	c := context.NewMockDefault()

	//testing when platform provider throws error
	osInfoProvider = MockPlatformInfoProviderReturningError
	getApplicationData = MockGetApplicationDataForAmazonLinux

	data = CollectAWSComponentData(c)

	assert.Equal(t, 0, len(data), "When Platform provider throws error - awscomponent data set must be empty")

	//testing for amazonLinux platform
	getApplicationData = MockGetApplicationDataForAmazonLinux
	osInfoProvider = MockPlatformInfoProviderReturningAmazonLinux

	data = CollectAWSComponentData(c)
	assert.Equal(t, len(sampleAWSComponentDataForAmazonLinux), len(data), "Given sample data for AmazonLinux is not returning data as expected")

	//testing for linux os other than amazonLinux
	getApplicationData = MockGetApplicationDataForLinuxOSOtherThanAmazonLinux
	osInfoProvider = MockPlatformInfoProviderReturningUbuntu

	data = CollectAWSComponentData(c)
	assert.Equal(t, len(sampleAWSComponentDataForLinuxOtherThanAmazonLinux), len(data), "Given sample data for Linux OS other than Amazon Linux is not returning data as expected")

	//testing for windows
	getApplicationData = MockGetApplicationDataForWindows
	osInfoProvider = MockPlatformInfoProviderReturningWindows

	data = CollectAWSComponentData(c)
	assert.Equal(t, len(sampleAWSComponentDataForWindows), len(data), "Given sample data for Windows is not returning data as expected")
}
