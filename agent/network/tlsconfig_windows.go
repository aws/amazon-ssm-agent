// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package network

import (
	"crypto/x509"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

func getSystemCertPool() (*x509.CertPool, error) {
	// Issue 16736, 18609:
	// System certificate pool not supported on windows
	return nil, nil
}

func getCustomCertificate(appConfig appconfig.SsmagentConfig) ([]byte, error) {
	// Custom Certificates not supported on windows
	return nil, nil
}
