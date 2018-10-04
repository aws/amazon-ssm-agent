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

// Package testdata represents the static data needed by agent tests
package testdata

var CrashWorkerErrorMessage = "document process failed unexpectedly:"

var CrashWorkerMDSMessage = `{
  "Parameters": null,
  "DocumentContent": {
    "schemaVersion": "2.2",
    "description": "doc",
    "runtimeConfig": null,
    "mainSteps": [
      {
        "action": "aws:runShellScript",
        "inputs": {
          "runCommand": [
            "kill -9 $PPID"
          ]
        },
        "maxAttempts": 0,
        "name": "pluginLinux",
        "onFailure": "",
        "settings": null,
        "timeoutSeconds": 0,
        "precondition": {
          "StringEquals": [
            "platformType",
            "Linux"
          ]
        }
      },
      {
        "action": "aws:runPowerShellScript",
        "inputs": {
          "runCommand": [
            "Taskkill /F /IM ssm-document-worker.exe"
          ]
        },
        "maxAttempts": 0,
        "name": "pluginWindows",
        "onFailure": "",
        "settings": null,
        "timeoutSeconds": 0,
        "precondition": {
          "StringEquals": [
            "platformType",
            "Windows"
          ]
        }
      }
    ],
    "parameters": null
  },
  "CommandId": "be8d9d4b-da53-4d2f-a96b-60aec17739af",
  "DocumentName": "test",
  "OutputS3KeyPrefix": "",
  "OutputS3BucketName": "",
  "CloudWatchLogGroupName": "",
  "CloudWatchOutputEnabled": "false"
  }`
