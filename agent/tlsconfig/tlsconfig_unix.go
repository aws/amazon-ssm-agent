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

// +build freebsd linux netbsd openbsd

package tlsconfig

import (
	"crypto/x509"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/tlsconfig/certreader"
)

func getSystemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}

func getCustomCertificate() ([]byte, error) {

	config, err := appconfig.Config(false)
	if err != nil {
		return nil, err
	}

	if !config.Agent.ContainerMode {
		return nil, fmt.Errorf("Custom certificate only allowed in container mode")
	}

	return certreader.ReadCertificate(appconfig.CustomCertificatePath)
}
