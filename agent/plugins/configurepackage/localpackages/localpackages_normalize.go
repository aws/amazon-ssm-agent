// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package localpackages implements the local storage for packages managed by the ConfigurePackage plugin.
package localpackages

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"strings"
)

// Names start with an ASCII letter or number and end with an ASCII letter, number, dash, plus, equals sign,
// grouping characters ()[]{}, or single dots
// Names may contain spaces but may not start or end with spaces
const nameValidChars = `\ \w+=()\[\]\{\}-`
const nameRegEx = `^(?:[a-zA-Z0-9])(?:[` + nameValidChars + `]*)(?:[.][` + nameValidChars + `]+)*$`
const nameMaxLength = 255
const dirMaxLength = 255

var nameRegExpValidator = regexp.MustCompile(nameRegEx)

// Return a name value that can be used as a directory name and is uniquely computable from the original name
// Alphanumeric names, dot-separated numeric versions, and semver-compliant versions less than 255 characters will all survive normalization unchanged
func normalizeDirectory(name string) string {
	if len(name) > nameMaxLength || !nameRegExpValidator.MatchString(name) || strings.HasSuffix(name, " ") {
		return generateDirectoryName(strings.ToLower(name))
	}
	return name // NOTE: backward compatibility with older agents requires we maintain case sensitivity for linux filesystems
}

// Return a safe, valid directory name that is similar to the original unsafe or invalid name and unique
// Start with "_" which means the name has been normalized, use "_" to separate the normalized string from the encoded hash suffix
func generateDirectoryName(directory string) string {
	lenOrig := len(directory)
	// Generate hash of original value and base64encode
	hash := sha256.Sum256([]byte(directory))
	hashSlice := hash[:]
	// Use an encoding that won't allows collisions on case-insensitive filesystems and include length to limit potential for maliciously generated collisions
	suffix := "_" + fmt.Sprintf("%X", lenOrig) + "_" + base32.StdEncoding.EncodeToString(hashSlice)
	// Replace all dots with dashes (so versions still have separation but we don't allow path traversal)
	normal := strings.Replace(directory, ".", "-", -1)
	// Remove all non alphanumeric characters
	normal = regexp.MustCompile("[^0-9a-zA-Z-]").ReplaceAllString(normal, "")
	// Shorten if necessary and append length and hash suffix
	if len(normal) > dirMaxLength-len(suffix) {
		normal = normal[0 : dirMaxLength-len(suffix)-1]
	}
	return "_" + normal + suffix
}
