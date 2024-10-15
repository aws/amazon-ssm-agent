package channel

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	commProtocol "github.com/aws/amazon-ssm-agent/common/channel/protocol"
	"github.com/aws/amazon-ssm-agent/common/channel/protocol/mocks"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type FileChannelTestSuite struct {
	suite.Suite
	mockLog log.T
}

// Execute the test suite
func TestFileChannelTestSuite(t *testing.T) {
	suite.Run(t, new(FileChannelTestSuite))
}

func (suite *FileChannelTestSuite) SetupTest() {
	mockLog := logmocks.NewMockLog()
	suite.mockLog = mockLog
	mockSurvey := &mocks.ISurvey{}
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	mockRespondent := &mocks.IRespondent{}
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}
}

func (suite *FileChannelTestSuite) TestInitialize_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")

	mockSurvey := &mocks.ISurvey{}
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn = NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized = fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
}

func (suite *FileChannelTestSuite) TestInitialize_Failure() {
	identityMock := &identityMocks.IAgentIdentity{}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize("")
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.False(suite.T(), isInitialized, "Initialization success")
}

func (suite *FileChannelTestSuite) TestRespondentDial_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	mockRespondent.On("Dial", mock.Anything).Return(nil)
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}

	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	fileChannelConn.Dial("test")
	isDialSuccessFul := fileChannelConn.IsDialSuccessful()
	assert.True(suite.T(), isDialSuccessFul, "Dialing unsuccessful for respondent")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *FileChannelTestSuite) TestSurveyorListen_Success() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	mockSurvey.On("Listen", mock.Anything).Return(nil)
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn := NewFileChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	fileChannelConn.Listen("test")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.True(suite.T(), isListenSuccessFul, "Listening unsuccessful for surveyor")
	isDialSuccess := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccess, "Dialing successful for surveyor")
}

func (suite *FileChannelTestSuite) TestRespondentDial_Failed() {
	identityMock := &identityMocks.IAgentIdentity{}
	mockRespondent := &mocks.IRespondent{}
	mockRespondent.On("Dial", mock.Anything).Return(nil)
	getRespondentInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.IRespondent {
		return mockRespondent
	}

	fileChannelConn := NewNamedPipeChannel(suite.mockLog, identityMock)
	fileChannelConn.Initialize(utils.Respondent)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for respondent")
	fileChannelConn.Dial("test")
	isDialSuccessFul := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessFul, "Dialing successful for respondent")
	isListenSuccessFul := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessFul, "Listening successful for respondent")
}

func (suite *FileChannelTestSuite) TestSurveyorListen_Failed() {
	identityMocks := &identityMocks.IAgentIdentity{}
	mockSurvey := &mocks.ISurvey{}
	mockSurvey.On("Listen", mock.Anything).Return(nil)
	getSurveyInstance = func(log log.T, identity identity.IAgentIdentity) commProtocol.ISurvey {
		return mockSurvey
	}
	fileChannelConn := NewNamedPipeChannel(suite.mockLog, identityMocks)
	fileChannelConn.Initialize(utils.Surveyor)
	isInitialized := fileChannelConn.IsChannelInitialized()
	assert.True(suite.T(), isInitialized, "Initialization failed for surveyor")
	fileChannelConn.Listen("test")
	isListenSuccessful := fileChannelConn.IsListenSuccessful()
	assert.False(suite.T(), isListenSuccessful, "Listening successful for surveyor")
	isDialSuccessful := fileChannelConn.IsDialSuccessful()
	assert.False(suite.T(), isDialSuccessful, "Dialing successful for surveyor")
}
