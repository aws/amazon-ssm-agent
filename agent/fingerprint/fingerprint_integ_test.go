// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// +build integration

package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostnameInfo(t *testing.T) {
	hostname, err := hostnameInfo()
	assert.NoError(t, err, "expected no error fetching the hostname")
	assert.NotEmpty(t, hostname, "expected to fetch hostname")
}

func TestPrimaryIpInfo(t *testing.T) {
	ip, err := primaryIpInfo()
	assert.NoError(t, err, "expected no error fetching the primary ip")
	assert.NotEmpty(t, ip, "expected to fetch primary ip")
}

func TestMacAddrInfo(t *testing.T) {
	mac, err := macAddrInfo()
	assert.NoError(t, err, "expected no error fetching the mac addr")
	assert.NotEmpty(t, mac, "expected to fetch mac address")
}
