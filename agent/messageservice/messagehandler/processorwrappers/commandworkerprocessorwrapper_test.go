package processorwrappers

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	docInfo = contracts.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		MessageID:    "MessageID",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}

	docState = contracts.DocumentState{
		DocumentInformation: docInfo,
		DocumentType:        contracts.SendCommand,
	}
)

type CommandProcessorWrapperTestSuite struct {
	suite.Suite
	contextMock                    *context.Mock
	commmandWorkerProcessorWrapper *CommandWorkerProcessorWrapper
	documentResultChan             chan contracts.DocumentResult
	outputMap                      map[contracts.UpstreamServiceName]chan contracts.DocumentResult
}

func (suite *CommandProcessorWrapperTestSuite) SetupTest() {
	contextMock := context.NewMockDefault()
	suite.contextMock = contextMock
	suite.documentResultChan = make(chan contracts.DocumentResult)
	suite.outputMap = make(map[contracts.UpstreamServiceName]chan contracts.DocumentResult)
	suite.outputMap[contracts.MessageDeliveryService] = suite.documentResultChan

	workerConfigs := utils.LoadProcessorWorkerConfig(contextMock)
	var cmdProcessorWrapper *CommandWorkerProcessorWrapper
	for _, config := range workerConfigs {
		if config.WorkerName == utils.DocumentWorkerName {
			cmdProcessorWrapper = NewCommandWorkerProcessorWrapper(contextMock, config).(*CommandWorkerProcessorWrapper)
			break
		}
	}
	suite.commmandWorkerProcessorWrapper = cmdProcessorWrapper
}

func (suite *CommandProcessorWrapperTestSuite) TestInitialize() {
	err := suite.commmandWorkerProcessorWrapper.Initialize(suite.outputMap)
	assert.Nil(suite.T(), err)
}

func (suite *CommandProcessorWrapperTestSuite) TestGetName() {
	name := suite.commmandWorkerProcessorWrapper.GetName()
	assert.Equal(suite.T(), utils.CommandProcessor, name)
}

func (suite *CommandProcessorWrapperTestSuite) TestGetStartWorker() {
	worker := suite.commmandWorkerProcessorWrapper.GetStartWorker()
	assert.Equal(suite.T(), contracts.SendCommand, worker)
}

func (suite *CommandProcessorWrapperTestSuite) TestGetTerminateWorker() {
	worker := suite.commmandWorkerProcessorWrapper.GetTerminateWorker()
	assert.Equal(suite.T(), contracts.CancelCommand, worker)
}

func (suite *CommandProcessorWrapperTestSuite) TestPushToProcessor() {
	isDocumentAlreadyReceived = func(idemCtx context.T, message *contracts.DocumentState) bool {
		return false
	}
	errorCode := suite.commmandWorkerProcessorWrapper.PushToProcessor(docState)
	assert.Equal(suite.T(), processor.ErrorCode(""), errorCode)
}

func (suite *CommandProcessorWrapperTestSuite) TestPushToProcessorWithUnsupportedDoc() {
	docState.DocumentType = contracts.StartSession
	isDocumentAlreadyReceived = func(idemCtx context.T, message *contracts.DocumentState) bool {
		return false
	}
	errorCode := suite.commmandWorkerProcessorWrapper.PushToProcessor(docState)
	assert.Equal(suite.T(), processor.UnsupportedDocType, errorCode)
}

func (suite *CommandProcessorWrapperTestSuite) TestPushToProcessorDuplicateDoc() {
	isDocumentAlreadyReceived = func(idemCtx context.T, message *contracts.DocumentState) bool {
		return false
	}
	suite.commmandWorkerProcessorWrapper.PushToProcessor(docState)
	isDocumentAlreadyReceived = func(idemCtx context.T, message *contracts.DocumentState) bool {
		return true
	}
	errorCode := suite.commmandWorkerProcessorWrapper.PushToProcessor(docState)
	assert.Equal(suite.T(), processor.DuplicateCommand, errorCode)
}

func (suite *CommandProcessorWrapperTestSuite) TestListenReply() {
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "plugin",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults[""] = &pluginResult
	messageId := uuid.NewV4()
	result := contracts.DocumentResult{
		Status:          contracts.ResultStatusInProgress,
		PluginResults:   pluginResults,
		LastPlugin:      "",
		MessageID:       messageId.String(),
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}

	go suite.commmandWorkerProcessorWrapper.listenReply(suite.documentResultChan, suite.outputMap)
	suite.documentResultChan <- result
	select {
	case <-suite.documentResultChan:
		assert.True(suite.T(), true, "message should be passed")
	case <-time.After(100 * time.Millisecond):
		assert.Fail(suite.T(), "message should have been passed")
	}
}

func TestCommandProcessorWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(CommandProcessorWrapperTestSuite))
}
