package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintCWConfig_RemovesCreds(t *testing.T) {

	config := `{
	"EngineConfiguration": {
        "PollInterval": "00:00:01",
        "Components": [
            {
                "Id": "SystemEventLog",
                "FullName": "AWS.EC2.Windows.CloudWatch.EventLog.EventLogInputComponent,AWS.EC2.Windows.CloudWatch",
                "Parameters": {
                    "LogName": "System",
                    "Levels": "7"
                }
            },
            {
                "Id": "CloudWatchLogs",
                "FullName": "AWS.EC2.Windows.CloudWatch.CloudWatchLogsOutput,AWS.EC2.Windows.CloudWatch",
                "Parameters": {
                    "AccessKey": "ABCDKEY",
                    "SecretKey": "test",
                    "Region": "us-west-2",
                    "LogGroup": "groupname",
                    "LogStream": "{instance_id}"
                }
            },
            {
                "Id": "CloudWatch",
                "FullName": "AWS.EC2.Windows.CloudWatch.CloudWatch.CloudWatchOutputComponent,AWS.EC2.Windows.CloudWatch",
                "Parameters":
                {
                    "AccessKey": "ABCDKEY",
                    "SecretKey": "test",
                    "Region": "us-west-2",
                    "NameSpace": "Windows/Default25"
                }
            }
        ],
        "Flows": {
            "Flows":
            [
                "(ApplicationEventLog,SystemEventLog),CloudWatchLogs",
				"(PerformanceCounter,PerformanceCounter2), CloudWatch"
            ]
        }
    }
}`
	log := NewMockLog()
	newConfig := PrintCWConfig(config, log)
	assert.Contains(t, config, "ABCDKEY")
	assert.NotContains(t, newConfig, "ABCDKEY")
	assert.NotContains(t, newConfig, "test+")
}

func TestPrintCWConfig_NoEngineConfig(t *testing.T) {
	config := `{"IsEnabled" = true}`
	log := NewMockLog()
	newConfig := PrintCWConfig(config, log)

	assert.Contains(t, newConfig, `"Components": null`)
}

func TestPrintCWConfig_ComponentsMissing(t *testing.T) {

	config := `{
	"EngineConfiguration": {
        "PollInterval": "00:00:01",
        "Flows": {
            "Flows":
            [
                "(ApplicationEventLog,SystemEventLog),CloudWatchLogs",
				"(PerformanceCounter,PerformanceCounter2), CloudWatch"
            ]
        }
    }
}`
	log := NewMockLog()
	newConfig := PrintCWConfig(config, log)
	assert.Contains(t, newConfig, `"Components": null`)
	assert.Contains(t, newConfig, `"PollInterval": "00:00:01"`)
}
