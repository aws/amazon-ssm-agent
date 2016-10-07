package windowsUpdate

import (
	"testing"

	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testUpdate = []inventory.WindowsUpdateData{
	inventory.WindowsUpdateData{
		"KB000001", "Security Update", "Wednesday, October 15, 2014 12:00:00 AM", "ADMINISTRATOR",
	},
	inventory.WindowsUpdateData{
		"KB000002", "Update", "Friday, June 20, 2014 12:00:00 AM", "NT AUTHORITY SYSTEM",
	},
}

func testExecuteCommand(command string, args ...string) ([]byte, error) {

	output, _ := json.Marshal(testUpdate)
	return output, nil
}

func TestGatherer(t *testing.T) {
	contextMock := context.NewMockDefault()
	gatherer := Gatherer(contextMock)
	cmdExecutor = testExecuteCommand
	item, err := gatherer.Run(contextMock, inventory.Config{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(item))
	assert.Equal(t, GathererName, item[0].Name)
	assert.Equal(t, schemaVersionOfWindowsUpdate, item[0].SchemaVersion)
	assert.Equal(t, testUpdate, item[0].Content)
}
