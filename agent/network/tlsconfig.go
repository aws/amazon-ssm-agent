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

package network

import (
	"crypto/tls"
	"crypto/x509"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const maxRetryCount = 3

var tlsConfig *tls.Config

var mutex = sync.Mutex{}

// GetDefaultTLSConfig creates and returns a configured TLS config
func GetDefaultTLSConfig(log log.T, appConfig appconfig.SsmagentConfig) *tls.Config {
	var err error
	var cert []byte
	var certPool *x509.CertPool

	mutex.Lock()
	defer mutex.Unlock()

	if tlsConfig == nil {
		// Initialize tls config
		tlsConfig = &tls.Config{}
	}
	tlsConfigCopy := tlsConfig.Clone()

	retryCount := 0
	for retryCount < maxRetryCount {
		// Initialize certificate pool
		certPool, err = getSystemCertPool()

		if err == nil {
			break
		}
		retryCount += 1
	}

	if err != nil {
		log.Warnf("Failed to read system certificate pool: %s", err)
	} else if certPool != nil {
		retryCount = 0
		for retryCount < maxRetryCount {
			// Load custom certificate
			cert, err = getCustomCertificate(appConfig)

			if err == nil {
				break
			}
			retryCount += 1
		}

		if err != nil {
			log.Debugf("Not using custom certificate: %s", err)
		} else if cert != nil {
			if !certPool.AppendCertsFromPEM(cert) {
				log.Debugf("Failed to append custom certificate to certificate pool")
			}
		}

		// Assign cert pool to tls config
		tlsConfigCopy.RootCAs = certPool
	}

	return tlsConfigCopy
}
