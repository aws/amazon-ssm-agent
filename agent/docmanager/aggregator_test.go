package docmanager

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestPrepareRuntimeStatus(t *testing.T) {
	type testCase struct {
		Input  contracts.PluginResult
		Output contracts.PluginRuntimeStatus
	}

	testCases := []testCase{
		{
			Input: contracts.PluginResult{
				PluginName:     "aws:runScript",
				Code:           0,
				Status:         "Success",
				Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
				StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
				EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
				StandardError:  "error",
				StandardOutput: "output",
			},
			Output: contracts.PluginRuntimeStatus{
				Name:           "aws:runScript",
				Code:           0,
				Status:         "Success",
				Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
				StartDateTime:  "2015-07-09T23:23:39.019Z",
				EndDateTime:    "2015-07-09T23:23:39.023Z",
				StandardError:  "error",
				StandardOutput: "output",
			},
		},
	}

	// run test cases
	for _, tst := range testCases {
		// call our method under test
		runtimeStatus := prepareRuntimeStatus(logger, tst.Input)

		// check result
		assert.Equal(t, tst.Output, runtimeStatus)
	}

	// test that there is a runtime status on error
	pluginResult := contracts.PluginResult{Error: fmt.Errorf("Plugin failed with error code 1")}
	runtimeStatus := prepareRuntimeStatus(logger, pluginResult)
	assert.NotNil(t, runtimeStatus.Output)
	return
}

//TODO add test for DocumentStatusAggregator
func TestDocumentStatus(t *testing.T) {
	type testCase struct {
		Input  map[string]*contracts.PluginResult
		Output contracts.ResultStatus
	}
	testCases := []testCase{
		{
			Input: map[string]*contracts.PluginResult{
				"aws:runScript": &contracts.PluginResult{
					PluginName:     "aws:runScript",
					Code:           0,
					Status:         "Success",
					Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
					StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
					EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
					StandardError:  "error",
					StandardOutput: "output",
				},
			},
			Output: contracts.ResultStatusSuccess,
		},
		{
			Input: map[string]*contracts.PluginResult{
				"aws:runScript": &contracts.PluginResult{
					PluginName:     "aws:runScript",
					Code:           0,
					Status:         "SuccessAndReboot",
					Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
					StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
					EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
					StandardError:  "error",
					StandardOutput: "output",
				},
			},
			Output: contracts.ResultStatusSuccessAndReboot,
		},
		{
			Input: map[string]*contracts.PluginResult{
				"aws:runScript": &contracts.PluginResult{
					PluginName:     "aws:runScript",
					Code:           0,
					Status:         "Success",
					Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
					StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
					EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
					StandardError:  "error",
					StandardOutput: "output",
				},
				"aws:runPowerShellScript": &contracts.PluginResult{
					PluginName:     "aws:runScript",
					Code:           0,
					Status:         "Failed",
					Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
					StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
					EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
					StandardError:  "error",
					StandardOutput: "output",
				},
			},
			Output: contracts.ResultStatusFailed,
		},
	}
	for _, tstCase := range testCases {
		status1, _, _ := DocumentResultAggregator(logger, "aws:runScript", tstCase.Input)
		status2, _, _ := DocumentResultAggregator(logger, "", tstCase.Input)
		assert.Equal(t, status1, contracts.ResultStatusInProgress)
		assert.Equal(t, status2, tstCase.Output)
	}

}

func TestDocumentStatusCount(t *testing.T) {
	input := map[string]*contracts.PluginResult{
		"aws:runScript": &contracts.PluginResult{
			PluginName:     "aws:runScript",
			Code:           0,
			Status:         "Success",
			Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
			StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
			EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
			StandardError:  "error",
			StandardOutput: "output",
		},
		"aws:runPowerShellScript": &contracts.PluginResult{
			PluginName:     "aws:runScript",
			Code:           0,
			Status:         "Failed",
			Output:         "standard output of test case\n----------ERROR-------\nstandard error of test case",
			StartDateTime:  times.ParseIso8601UTC("2015-07-09T23:23:39.019Z"),
			EndDateTime:    times.ParseIso8601UTC("2015-07-09T23:23:39.023Z"),
			StandardError:  "error",
			StandardOutput: "output",
		},
	}
	output := map[string]int{
		"Success": 1,
		"Failed":  1,
	}
	_, statusCount, _ := DocumentResultAggregator(logger, "", input)
	assert.Equal(t, statusCount, output)
}
