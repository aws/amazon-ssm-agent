package iomodule

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type cloudWatchServiceRetrieverMock struct {
	mock.Mock
}

func (m *cloudWatchServiceRetrieverMock) NewCloudWatchLogsService(context context.T) ICloudWatchLoggingService {
	args := m.Called(context)
	return args.Get(0).(*cloudWatchLoggingServiceMock)
}

type cloudWatchLoggingServiceMock struct {
	mock.Mock
}

func (m *cloudWatchLoggingServiceMock) StreamData(
	logGroupName string,
	logStreamName string,
	absoluteFilePath string,
	isFileComplete bool,
	isLogStreamCreated bool,
	fileCompleteSignal chan bool,
	cleanupControlCharacters bool,
	structuredLogs bool) (success bool) {
	args := m.Called(logGroupName, logStreamName, absoluteFilePath, isFileComplete, isLogStreamCreated,
		fileCompleteSignal, cleanupControlCharacters, structuredLogs)

	return args.Bool(0)
}

func (m *cloudWatchLoggingServiceMock) SetIsFileComplete(val bool) {
	m.Called(val)
}

func (m *cloudWatchLoggingServiceMock) GetIsUploadComplete() bool {
	args := m.Called()
	return args.Bool(0)
}

type s3LogsServiceRetrieverMock struct {
	mock.Mock
}

func (m *s3LogsServiceRetrieverMock) NewAmazonS3Util(context context.T, outputS3BucketName string) (IS3Util, error) {
	args := m.Called(context, outputS3BucketName)
	return args.Get(0).(*s3UtilMock), args.Error(1)
}

type s3UtilMock struct {
	mock.Mock
}

func (m *s3UtilMock) S3Upload(log log.T, outputS3BucketName string, s3Key string, filePath string) error {
	args := m.Called(log, outputS3BucketName, s3Key, filePath)
	return args.Error(0)
}
