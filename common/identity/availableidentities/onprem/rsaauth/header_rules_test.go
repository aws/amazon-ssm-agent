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

// Copied from github.com/aws/aws-sdk-go/private/signer/v4
// to provide common SigV4 dependenies for the RSA signer.

package rsaauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleCheckWhitelist(t *testing.T) {
	w := whitelist{
		mapRule{
			"Cache-Control": struct{}{},
		},
	}

	assert.True(t, w.IsValid("Cache-Control"))
	assert.False(t, w.IsValid("Cache-"))
}

func TestRuleCheckBlacklist(t *testing.T) {
	b := blacklist{
		mapRule{
			"Cache-Control": struct{}{},
		},
	}

	assert.False(t, b.IsValid("Cache-Control"))
	assert.True(t, b.IsValid("Cache-"))
}

func TestRuleCheckPattern(t *testing.T) {
	p := patterns{"X-Amz-Meta-"}

	assert.True(t, p.IsValid("X-Amz-Meta-"))
	assert.True(t, p.IsValid("X-Amz-Meta-Star"))
	assert.False(t, p.IsValid("Cache-"))
}

func TestRuleComplexWhitelist(t *testing.T) {
	w := rules{
		whitelist{
			mapRule{
				"Cache-Control": struct{}{},
			},
		},
		patterns{"X-Amz-Meta-"},
	}

	r := rules{
		inclusiveRules{patterns{"X-Amz-"}, blacklist{w}},
	}

	assert.True(t, r.IsValid("X-Amz-Blah"))
	assert.False(t, r.IsValid("X-Amz-Meta-"))
	assert.False(t, r.IsValid("X-Amz-Meta-Star"))
	assert.False(t, r.IsValid("Cache-Control"))
}
