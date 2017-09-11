// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package ssmparameterresolver contains types and methods for resolving SSM Parameter references.
package ssmparameterresolver

import "regexp"

const (
	ssmNonSecurePrefix = "ssm:"
	ssmSecurePrefix    = "ssm-secure:"
	secureStringType   = "SecureString"
	stringType         = "String"

	// Maximum number of parameters that can be requested from SSM Parameter store in one GetParameters request
	maxParametersRetrievedFromSsm = 10
)

// SSM Parameter placeholder - relaxed regular expression
var ssmParameterPlaceholderRegEx = regexp.MustCompile("{{\\s*(" + ssmNonSecurePrefix + "[\\w-/]+)\\s*}}")
var secureSsmParameterPlaceholderRegEx = regexp.MustCompile("{{\\s*(" + ssmSecurePrefix + "[\\w-/]+)\\s*}}")

// SsmParameterInfo structure represents a resolved SSM Parameter.
type SsmParameterInfo struct {
	Name  string
	Type  string
	Value string
}

// ResolveOptions structure represents a set of options for the parameter resolution.
// At this time it has only one flag IgnoreSecureParameters
// if IgnoreSecureParameters == true the parameters prefixed with ssm-secure: will not be resolved.
type ResolveOptions struct {
	IgnoreSecureParameters bool
}
