// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package testutils represents the common logic needed for agent tests
package testdata

var EchoMDSMessage = `{
		"CommandId": "12345c50-d2b4-4380-acac-213fb7c12345",
		"Parameters": {
			"commands": [
				"echo ship_it"
			]
		},
		"DocumentContent": {
			"schemaVersion": "2.2",
			"description": "Cross-platform document",
			"mainSteps": [
				{
					"action": "aws:runShellScript",
					"precondition": {
						"StringEquals": ["platformType", "Linux"]
					},
					"inputs": {
						"commands": "date"
					},
					"maxAttempts": 0,
					"name": "runShellScript1",
					"onFailure": "",
					"settings": null,
					"timeoutSeconds": 0
				},
				{
					"action": "aws:runShellScript",
					"precondition": {
						"StringEquals": ["platformType", "Linux"]
					},
					"inputs": {
						"commands": "echo ship_it"
					},
					"maxAttempts": 0,
					"name": "runShellScript2",
					"onFailure": "",
					"settings": null,
					"timeoutSeconds": 0
				}
			],
			"parameters": {
				"commands": {
					"allowedPattern": "",
					"allowedValues": null,
					"default": null,
					"description": "(Required) Specify a shell script or a command to run.",
					"type": "StringList"
				}
			}
		},
		"DocumentName": "ship-it",
		"OutputS3BucketName": "",
		"OutputS3KeyPrefix": "",
		"CloudWatchLogGroupName": "",
		"CloudWatchOutputEnabled": "false"
	}`
