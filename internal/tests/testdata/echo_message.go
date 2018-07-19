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
		"Parameters": {
			"commands": [
				"echo ship_it"
			],
			"executionTimeout": "3600"
		},
		"DocumentContent": {
			"schemaVersion": "1.2",
			"description": "Run a Shell script or specify the paths to scripts to run.",
			"runtimeConfig": {
				"aws:runShellScript": {
					"settings": null,
					"properties": [
						{
							"id": "0.aws:runShellScript",
							"runCommand": [
								"echo ship_it"
							],
							"timeoutSeconds": "3600",
							"workingDirectory": ""
						}
					],
					"description": ""
				}
			},
			"mainSteps": null,
			"parameters": {
				"commands": {
					"default": null,
					"description": "(Required) Specify the commands to run or the paths to existing scripts on the instance.",
					"type": "StringList",
					"allowedValues": null,
					"allowedPattern": ""
				},
				"executionTimeout": {
					"default": "3600",
					"description": "(Optional) The time in seconds for a command to be completed before it is considered to have failed. Default is 3600 (1 hour). Maximum is 172800 (48 hours).",
					"type": "String",
					"allowedValues": null,
					"allowedPattern": "([1-9][0-9]{0,4})|(1[0-6][0-9]{4})|(17[0-1][0-9]{3})|(172[0-7][0-9]{2})|(172800)"
				},
				"workingDirectory": {
					"default": "",
					"description": "(Optional) The path to the working directory on your instance.",
					"type": "String",
					"allowedValues": null,
					"allowedPattern": ""
				}
			}
		},
		"CommandId": "767dc876-0713-405a-b397-266953163fd2",
		"DocumentName": "AWS-RunShellScript",
		"OutputS3KeyPrefix": "",
		"OutputS3BucketName": "",
		"CloudWatchLogGroupName": "",
		"CloudWatchOutputEnabled": "false"
	}`
