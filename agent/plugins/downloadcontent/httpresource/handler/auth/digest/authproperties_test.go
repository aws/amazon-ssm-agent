/*
 * Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package digest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWwwAuthenticateProps(t *testing.T) {
	tests := []struct {
		wwwAuthValue string
		authProps    *WwwAuthenticateProps
	}{
		{
			`realm="test@example.com"`,
			&WwwAuthenticateProps{
				realm: "test@example.com",
			},
		},
		{
			`nonce="8ed61ddf6084153223eb4e8108777472"`,
			&WwwAuthenticateProps{
				nonce: "8ed61ddf6084153223eb4e8108777472",
			},
		},
		{
			`opaque="6655c45fbc34d0d60f7dca67d78c69fb"`,
			&WwwAuthenticateProps{
				opaque: "6655c45fbc34d0d60f7dca67d78c69fb",
			},
		},
		{
			`qop="auth, auth-int"`,
			&WwwAuthenticateProps{
				qop: []string{"auth", "auth-int"},
			},
		},
		{
			`algorithm=MD5`,
			&WwwAuthenticateProps{
				algorithm: "MD5",
			},
		},
		{
			`userhash=true`,
			&WwwAuthenticateProps{
				userhash: true,
			},
		},
		{
			`Digest realm="realm", nonce="nonce", qop="auth, auth-int", opaque="opaque", algorithm=MD5, userhash=false`,
			&WwwAuthenticateProps{
				realm:     "realm",
				nonce:     "nonce",
				opaque:    "opaque",
				algorithm: "MD5",
				qop:       []string{"auth", "auth-int"},
				userhash:  false,
			},
		},
	}

	for _, test := range tests {
		actualAuthzProps := newWwwAuthenticateProps(test.wwwAuthValue)

		if test.authProps.qop == nil {
			test.authProps.qop = []string{"auth"}
		}

		if test.authProps.algorithm == "" {
			test.authProps.algorithm = MD5
		}

		assert.Equal(t, test.authProps, actualAuthzProps)
	}
}
