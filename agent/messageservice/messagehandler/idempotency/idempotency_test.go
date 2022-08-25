package idempotency

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	docInfo = contracts.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		MessageID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}
	docState = &contracts.DocumentState{
		DocumentInformation: docInfo,
		DocumentType:        contracts.SendCommand,
	}
	mockContext = context.NewMockDefault()
)

type IdeopotencyTestSuite struct {
	suite.Suite
	mockContext *context.Mock
}

func (suite *IdeopotencyTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	suite.mockContext = mockContext
	persistenceTimeoutMinutes = 0 //  set to 0 for testing to prevent a longer build time
}

func (suite *IdeopotencyTestSuite) TestIdempotency_CreateNewEntry() {
	makeDirs = func(destinationDir string) (err error) {
		return nil
	}
	err := CreateIdempotencyEntry(suite.mockContext, docState)
	assert.Equal(suite.T(), nil, err)
}

func (suite *IdeopotencyTestSuite) TestIdempotency_IsDocumentAlreadyReceived_WhenItDoesntExist() {
	isNotExist = func(err error) bool {
		return true
	}
	exists := IsDocumentAlreadyReceived(suite.mockContext, docState)
	assert.False(suite.T(), exists)
}

func (suite *IdeopotencyTestSuite) TestIdempotency_IsDocumentAlreadyReceived_WhenItDoesExist() {
	isNotExist = func(err error) bool {
		return false
	}
	exists := IsDocumentAlreadyReceived(suite.mockContext, docState)
	assert.True(suite.T(), exists)
}

func (suite *IdeopotencyTestSuite) TestIdempotency_CleanupOldIdempotencyEntries() {
	getDirectoryUnsortedOlderThan = func(srcPath string, olderThan *time.Time) (strings []string, err error) {
		return []string{"dir1", "dir2"}, nil
	}
	getIdempotencyDir = func(context context.T) string {
		return "c:"
	}
	files := make(map[string]bool)
	files[filepath.Join("c:", "dir1", "dir2")] = true
	files[filepath.Join("c:", "dir2", "dir1")] = true
	files[filepath.Join("c:", "dir1", "dir1")] = true
	files[filepath.Join("c:", "dir2", "dir2")] = true

	deleteDirectory = func(dirName string) (err error) {
		delete(files, dirName)
		return nil
	}
	CleanupOldIdempotencyEntries(suite.mockContext)
	assert.Equal(suite.T(), 0, len(files))
}

// Execute the test suite
func TestMessageHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(IdeopotencyTestSuite))
}
