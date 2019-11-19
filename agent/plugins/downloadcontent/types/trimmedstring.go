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

// Package types defines custom types
package types

import (
	"bytes"
	"strings"
)

// TrimmedString is a string with no leading or trailing spaces
type TrimmedString string

// UnmarshalJSON unmarshalls a given value to  TrimmedString
func (field *TrimmedString) UnmarshalJSON(rawBytes []byte) error {
	trimmedStr := bytes.TrimSpace(bytes.Trim(rawBytes, `"`))
	*field = TrimmedString(trimmedStr)
	return nil
}

// Val returns the string value of a TrimmedString object
func (field *TrimmedString) Val() string {
	return string(*field)
}

// NewTrimmedString creates a new TrimmedString object based on a given string
func NewTrimmedString(val string) TrimmedString {
	return TrimmedString(strings.TrimSpace(val))
}
