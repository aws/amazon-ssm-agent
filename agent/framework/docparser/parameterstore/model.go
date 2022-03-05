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

// Package parameterstore contains modules to resolve ssm parameters present in the document.
package parameterstore

// GetParametersResponse represents GetParameters API response
type GetParametersResponse struct {
	Parameters        []Parameter
	InvalidParameters []string
}

// GetParameterResponse represents the response from GetParameter API
type GetParameterResponse struct {
	Response Parameter `json:"Parameter"`
}

// Parameter contains info about the parameter
type Parameter struct {
	Name    string
	Type    string
	Value   string
	Version int64
}
