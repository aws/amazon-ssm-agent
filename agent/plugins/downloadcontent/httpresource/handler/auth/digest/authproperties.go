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

// Package digest defines functionality required to support digest authorization
package digest

import (
	"regexp"
	"strings"
)

// WwwAuthenticateProps defines all supported WWW-Authenticate response header field attributes
// https://tools.ietf.org/html/rfc7616#section-3.3
type WwwAuthenticateProps struct {
	realm     string
	nonce     string
	opaque    string
	algorithm string
	qop       []string
	userhash  bool
}

var realmRegex = regexp.MustCompile(`realm="(.+?)"`)
var nonceRegex = regexp.MustCompile(`nonce="(.+?)"`)
var opaqueRegex = regexp.MustCompile(`opaque="(.+?)"`)
var algorithmRegex = regexp.MustCompile(`algorithm=([^, ]+)`)
var qopRegex = regexp.MustCompile(`qop="(.+?)"`)
var userhashRegex = regexp.MustCompile(`userhash=(true|false)`)

// newWwwAuthenticateProps creates a WwwAuthenticateProps instance based on the WWW-Authenticate field value
func newWwwAuthenticateProps(wwwAuthValue string) *WwwAuthenticateProps {
	props := WwwAuthenticateProps{}

	if match := realmRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.realm = match[1]
	}

	if match := nonceRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.nonce = match[1]
	}

	if match := opaqueRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.opaque = match[1]
	}

	if match := algorithmRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.algorithm = match[1]
	} else {
		props.algorithm = MD5
	}

	if match := qopRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.qop = strings.Split(match[1], ",")
		for i := range props.qop {
			props.qop[i] = strings.TrimSpace(props.qop[i])
		}
	} else {
		props.qop = []string{"auth"}
	}

	if match := userhashRegex.FindStringSubmatch(wwwAuthValue); match != nil {
		props.userhash = strings.ToLower(match[1]) == "true"
	}

	return &props
}
