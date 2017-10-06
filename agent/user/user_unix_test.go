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

// +build darwin dragonfly freebsd !android,linux netbsd openbsd solaris

// package user re-implements os/user functions without the use of cgo for unix
package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUser(t *testing.T) {
	user, err := parsePasswdUser("root:*:0:0:root:/root:/bin/sh")

	assert.Nil(t, err)
	assert.Equal(t, user.Username, "root")
	assert.Equal(t, user.Uid, "0")
	assert.Equal(t, user.Gid, "0")
	assert.Equal(t, user.Name, "root")
	assert.Equal(t, user.HomeDir, "/root")
}

func TestParseUser_MissingField(t *testing.T) {
	_, err := parsePasswdUser("root:*:0:0:/root:/bin/sh")
	assert.NotNil(t, err)
}

func TestParseUser_InvalidGID(t *testing.T) {
	_, err := parsePasswdUser("root:*:a:0:root:/root:/bin/sh")
	assert.NotNil(t, err)
}

func TestParseUser_InvalidUID(t *testing.T) {
	_, err := parsePasswdUser("root:*:0:a:root:/root:/bin/sh")
	assert.NotNil(t, err)
}

func TestParseUser_InvalidFormat(t *testing.T) {
	_, err := parsePasswdUser("root-*-0-0-root-/root-/bin/sh")
	assert.NotNil(t, err)
}
