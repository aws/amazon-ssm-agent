// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

package diagnostics

import (
	"fmt"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceCheckStrServiceManagerFailed = "Failed to connect to window service manager: %v"
	serviceCheckStrOpenServiceFailed    = "Failed to open the agent service: %v"
	serviceCheckStrQueryServiceFailed   = "Failed to query the agent service status: %v"
	serviceCheckStrServiceNotRunning    = "Agent service is not running, state is: %s"
)

func windowsServiceStatusToString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "StartPending"
	case svc.StopPending:
		return "StopPending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "ContinuePending"
	case svc.PausePending:
		return "PausePending"
	case svc.Paused:
		return "Paused"
	default:
		return ""
	}
}

func isServiceRunning() error {
	serviceName := "AmazonSSMAgent"

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf(serviceCheckStrServiceManagerFailed, err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf(serviceCheckStrOpenServiceFailed, err)
	}
	defer service.Close()

	serviceStatus, err := service.Query()
	if err != nil {
		return fmt.Errorf(serviceCheckStrQueryServiceFailed, err)
	}

	if serviceStatus.State != svc.Running {
		stateStr := windowsServiceStatusToString(serviceStatus.State)
		return fmt.Errorf(serviceCheckStrServiceNotRunning, stateStr)
	}

	return nil
}
