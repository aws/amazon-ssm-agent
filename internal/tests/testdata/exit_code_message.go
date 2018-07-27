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

var ZeroExitCodeMessage = `{
  "Parameters": {
    "commands": [
      "echo This message goes to stdout",
      "echo This message goes to stderr \u003e\u00262"
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
              "echo This message goes to stdout",
              "echo This message goes to stderr \u003e\u00262"
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
  "CommandId": "9fc4ecf0-9b2a-4670-8f49-34d1dbaa71c4",
  "DocumentName": "AWS-RunShellScript",
  "OutputS3KeyPrefix": "",
  "OutputS3BucketName": "",
  "CloudWatchLogGroupName": "",
  "CloudWatchOutputEnabled": "false"
}`

var ZeroExitCodeMessage_Windows = `{
  "Parameters": {
    "commands": [
      "echo 'This message goes to stdout'",
      "[System.Console]::Error.WriteLine('This message goes to stderr')"
    ],
    "executionTimeout": "3600"
  },
  "DocumentContent": {
    "schemaVersion": "1.2",
    "description": "Run a PowerShell script or specify the paths to scripts to run.",
    "runtimeConfig": {
      "aws:runPowerShellScript": {
        "settings": null,
        "properties": [
          {
            "id": "0.aws:runPowerShellScript",
            "runCommand": [
              "echo 'This message goes to stdout'",
              "[System.Console]::Error.WriteLine('This message goes to stderr')"
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
  "CommandId": "9fc4ecf0-9b2a-4670-8f49-34d1dbaa71c4",
  "DocumentName": "AWS-RunPowerShellScript",
  "OutputS3KeyPrefix": "",
  "OutputS3BucketName": "",
  "CloudWatchLogGroupName": "",
  "CloudWatchOutputEnabled": "false"
}`

var ZeroExitCodeMessageV2 = `{
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
              "echo This message goes to stdout",
              "echo This message goes to stderr \u003e\u00262"
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
              "echo 'This message goes to stdout'",
              "[System.Console]::Error.WriteLine('This message goes to stderr')"
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

var NonZeroExitCodeMessageV2 = `{
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
          "echo This message goes to stdout",
          "echo This message goes to stderr \u003e\u00262",
          "exit 2"
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
          "echo 'This message goes to stdout'",
          "[System.Console]::Error.WriteLine('This message goes to stderr')",
          "exit 2"
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

var NonZeroExitCodeMessage = `{
  "Parameters": {
    "commands": [
      "echo This message goes to stdout",
      "echo This message goes to stderr \u003e\u00262",
      "exit 2"
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
              "echo This message goes to stdout",
              "echo This message goes to stderr \u003e\u00262",
              "exit 2"
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
  "CommandId": "9fc4ecf0-9b2a-4670-8f49-34d1dbaa71c4",
  "DocumentName": "AWS-RunShellScript",
  "OutputS3KeyPrefix": "",
  "OutputS3BucketName": "",
  "CloudWatchLogGroupName": "",
  "CloudWatchOutputEnabled": "false"
}`

var NonZeroExitCodeMessage_Windows = `{
  "Parameters": {
    "commands": [
      "echo This message goes to stdout",
      "[System.Console]::Error.WriteLine('This message goes to stderr')",
      "exit 2"
    ],
    "executionTimeout": "3600"
  },
  "DocumentContent": {
    "schemaVersion": "1.2",
    "description": "Run a Power Shell script or specify the paths to scripts to run.",
    "runtimeConfig": {
      "aws:runPowerShellScript": {
        "settings": null,
        "properties": [
          {
            "id": "0.aws:runPowerShellScript",
            "runCommand": [
              "echo 'This message goes to stdout'",
              "[System.Console]::Error.WriteLine('This message goes to stderr')",
              "exit 2"
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
  "CommandId": "9fc4ecf0-9b2a-4670-8f49-34d1dbaa71c4",
  "DocumentName": "AWS-RunPowerShellScript",
  "OutputS3KeyPrefix": "",
  "OutputS3BucketName": "",
  "CloudWatchLogGroupName": "",
  "CloudWatchOutputEnabled": "false"
}`
var CommandStdout = "This message goes to stdout"
var CommandStderr = "This message goes to stderr"
var ZeroExitCode = 0
var NonZeroExitCode = 2
