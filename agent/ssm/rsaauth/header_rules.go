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
//
//Copied from github.com/aws/aws-sdk-go/private/signer/v4
// to provide common SigV4 dependenies for the RSA signer.

package rsaauth

import (
	"net/http"
	"strings"
)

// validator houses a set of rule needed for validation of a
// string value
type rules []rule

// rule interface allows for more flexible rules and just simply
// checks whether or not a value adheres to that rule
type rule interface {
	IsValid(value string) bool
}

// IsValid will iterate through all rules and see if any rules
// apply to the value and supports nested rules
func (r rules) IsValid(value string) bool {
	for _, rule := range r {
		if rule.IsValid(value) {
			return true
		}
	}
	return false
}

// mapRule generic rule for maps
type mapRule map[string]struct{}

// IsValid for the map rule satisfies whether it exists in the map
func (m mapRule) IsValid(value string) bool {
	_, ok := m[value]
	return ok
}

// whitelist is a generic rule for whitelisting
type whitelist struct {
	rule
}

// IsValid for whitelist checks if the value is within the whitelist
func (w whitelist) IsValid(value string) bool {
	return w.rule.IsValid(value)
}

// blacklist is a generic rule for blacklisting
type blacklist struct {
	rule
}

// IsValid for whitelist checks if the value is within the whitelist
func (b blacklist) IsValid(value string) bool {
	return !b.rule.IsValid(value)
}

type patterns []string

// IsValid for patterns checks each pattern and returns if a match has
// been found
func (p patterns) IsValid(value string) bool {
	for _, pattern := range p {
		if strings.HasPrefix(http.CanonicalHeaderKey(value), pattern) {
			return true
		}
	}
	return false
}

// inclusiveRules rules allow for rules to depend on one another
type inclusiveRules []rule

// IsValid will return true if all rules are true
func (r inclusiveRules) IsValid(value string) bool {
	for _, rule := range r {
		if !rule.IsValid(value) {
			return false
		}
	}
	return true
}
