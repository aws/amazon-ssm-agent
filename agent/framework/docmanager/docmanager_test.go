// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package docmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	TEST_ORC_DIR = "orchestration"
	TEST_SEC_DIR = "session"
	TEST_DOC_DIR = "document"
	TIMING_TEST  = "timing_test"
)

func TestDocManagerLock(t *testing.T) {
	acquired := getLock(TEST_ORC_DIR)
	assert.True(t, acquired)

	acquired = getLock(TEST_ORC_DIR)
	assert.False(t, acquired)
	releaseLock(TEST_ORC_DIR)

	acquired = getLock(TEST_ORC_DIR)
	assert.True(t, acquired)
	releaseLock(TEST_ORC_DIR)
}

func TestDocManagerLockTiming(t *testing.T) {
	acquired := getLock(TIMING_TEST)
	assert.True(t, acquired)
	updateTime(TIMING_TEST)
	releaseLock(TIMING_TEST)

	acquired = getLock(TIMING_TEST)
	assert.False(t, acquired)
}

func getAndLock(name string, dc chan int, t *testing.T) {
	acquired := getLock(name)
	assert.True(t, acquired)
	updateTime(name)
	releaseLock(name)
	dc <- 0
}

func TestDocManagerLockMultipleFolders(t *testing.T) {
	dc := make(chan int)
	go getAndLock(TEST_DOC_DIR, dc, t)
	go getAndLock(TEST_SEC_DIR, dc, t)
	<-dc
	<-dc
	assert.False(t, getLock(TEST_DOC_DIR))
	assert.False(t, getLock(TEST_SEC_DIR))
}
