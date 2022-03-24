package testcases

import (
	"testing"

	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/mocks"
	"github.com/stretchr/testify/assert"
)

func TestEc2DetectorTestCase_ExecuteTestCase_DetectorTrue(t *testing.T) {
	detector := &mocks.Ec2Detector{}
	tc := Ec2DetectorTestCase{detector: detector}

	// Test when detector returns true
	detector.On("IsEC2Instance").Return(true).Once()
	output := tc.ExecuteTestCase()
	assert.Equal(t, testCommon.TestCasePass, output.Result)

	// Assert detector was called
	detector.AssertExpectations(t)
}

func TestEc2DetectorTestCase_ExecuteTestCase_DetectorFalse(t *testing.T) {
	detector := &mocks.Ec2Detector{}
	tc := Ec2DetectorTestCase{detector: detector}

	detector.On("IsEC2Instance").Return(false).Once()
	output := tc.ExecuteTestCase()
	assert.Equal(t, testCommon.TestCaseFail, output.Result)

	// Assert detector was called
	detector.AssertExpectations(t)
}
