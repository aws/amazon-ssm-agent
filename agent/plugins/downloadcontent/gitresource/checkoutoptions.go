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

// Package gitresource implements methods and defines resources required to access git repositories
package gitresource

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/types"
)

const (
	defaultBranch = "master"
)

// CheckoutOptions defines the attributes allowed to be passed to the internal `git checkout` operation
type CheckoutOptions struct {
	Branch   types.TrimmedString
	CommitID types.TrimmedString
}

// ParseCheckoutOptions extracts repository get content options which can be a commit ID or branch name
func ParseCheckoutOptions(log log.T, checkoutOptions string) (*CheckoutOptions, error) {
	if checkoutOptions == "" {
		return &CheckoutOptions{}, nil
	}

	log.Debug("Splitting getOptions to get the actual options - ", checkoutOptions)
	branchOrSHA := strings.Split(checkoutOptions, ":")
	if len(branchOrSHA) == 2 {
		if strings.Compare(branchOrSHA[0], "branch") != 0 && strings.Compare(branchOrSHA[0], "commitID") != 0 {
			return nil, errors.New("Type of option is unknown. Please use either 'branch' or 'commitID'.")
		}
		//Error if extra option has been specified but is empty
		// Length must be 2 (key and value)
		if branchOrSHA[1] == "" {
			return nil, errors.New("Option for retrieving git content is empty")
		}
	} else if len(branchOrSHA) > 2 {
		return nil, errors.New("Only specify one required option")
	} else {
		return nil, errors.New("getOptions is not specified in the right format")
	}
	log.Info("GetOptions value - ", branchOrSHA[1])

	value := types.NewTrimmedString(branchOrSHA[1])

	options := CheckoutOptions{}
	switch branchOrSHA[0] {
	case "commitID":
		options.CommitID = value
		break
	case "branch":
		options.Branch = value
		break
	}

	return &options, nil
}
