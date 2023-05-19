// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package domainjoin implements the domain join plugin.
package domainjoin

// Returns true if an injection command is detected in a shell command
func isShellInjection(arg string) bool {
	// The following characters are common shell injection characters
	// Keeping the checks separate for readability

	injectionCharsArray := []byte{'$', '`', ';', '|', '&'}

	for _, c := range injectionCharsArray {
		for _, s := range arg {
			if byte(s) == c {
				return true
			}
		}
	}

	return false
}
