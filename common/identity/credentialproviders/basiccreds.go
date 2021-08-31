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

package credentialproviders

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
)

func GetRemoteCreds() *credentials.Credentials {
	cfg := defaults.Config()
	handlers := defaults.Handlers()
	remoteCreds := defaults.RemoteCredProvider(*cfg, handlers)

	return credentials.NewCredentials(remoteCreds)
}

func GetDefaultCreds() *credentials.Credentials {
	cfg := defaults.Config()
	handlers := defaults.Handlers()

	return defaults.CredChain(cfg, handlers)
}
