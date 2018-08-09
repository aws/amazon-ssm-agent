// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package rebooter

import (
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define Rebooter TestSuite struct
type RebooterTestSuite struct {
	suite.Suite
	rebooter IRebootType
	logMock  *log.Mock
}

//Initialize the rebooter test suite struct
func (suite *RebooterTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	suite.logMock = logMock
	suite.rebooter = &SSMRebooter{}
}

// Test function for PendingReboot
func (suite *RebooterTestSuite) TestRequestPendingReboot() {
	var successCount int = 0
	var wg sync.WaitGroup
	go func(log log.T) {
		ch = suite.rebooter.GetChannel()
		val := <-ch
		if val == RebootRequestTypeReboot {
			log.Info("start rebooting the machine...")
		} else {
			log.Error("reboot type not supported yet")
		}
	}(suite.logMock)
	time.Sleep(200 * time.Millisecond)
	// Random number
	total := 10
	// Spawn goroutines to Request Pending Reboot
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if RequestPendingReboot(suite.logMock) {
				successCount++
			}
		}()
	}
	wg.Wait()
	// Wait a second to allow some ops to accumulate.
	time.Sleep(time.Second)
	// Request reboot should only return true once
	assert.Equal(suite.T(), successCount, 1, "Request reboot should only return true once")
}

func TestRebooterTestSuite(t *testing.T) {
	suite.Run(t, new(RebooterTestSuite))
}
