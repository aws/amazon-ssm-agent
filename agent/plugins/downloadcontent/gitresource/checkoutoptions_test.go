/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package gitresource

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logMock = log.NewMockLog()

func TestParseGetOptions_CommitId(t *testing.T) {
	getOptions := "commitID:123"
	options, err := ParseCheckoutOptions(logMock, getOptions)

	assert.NoError(t, err)
	assert.Equal(t, &CheckoutOptions{
		CommitID: "123",
	}, options)
}

func TestParseGetOptions_Branch(t *testing.T) {
	getOptions := "branch:feature-a"
	options, err := ParseCheckoutOptions(logMock, getOptions)

	assert.NoError(t, err)
	assert.Equal(t, &CheckoutOptions{
		Branch: "feature-a",
	}, options)
}

func TestParseGetOptions_NoOptions(t *testing.T) {
	getOptions := ""
	options, err := ParseCheckoutOptions(logMock, getOptions)

	assert.NoError(t, err)
	assert.Equal(t, &CheckoutOptions{}, options)
}

func TestParseGetOptions_ErrorUnknownOptions(t *testing.T) {
	getOptions := "test:12"
	_, err := ParseCheckoutOptions(logMock, getOptions)

	assert.Error(t, err)
	assert.Equal(t, "Type of option is unknown. Please use either 'branch' or 'commitID'.", err.Error())
}

func TestParseGetOptions_ErrorContentEmpty(t *testing.T) {
	getOptions := "branch:"
	_, err := ParseCheckoutOptions(logMock, getOptions)

	assert.Error(t, err)
	assert.Equal(t, "Option for retrieving git content is empty", err.Error())
}

func TestParseGetOptions_ErrorMultiValues(t *testing.T) {
	getOptions := "branch:master,commitID:123"
	_, err := ParseCheckoutOptions(logMock, getOptions)

	assert.Error(t, err)
	assert.Equal(t, "Only specify one required option", err.Error())
}
