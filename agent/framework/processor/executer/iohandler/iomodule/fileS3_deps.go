package iomodule

import (
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
)

var cloudWatchServiceRetriever ICWServiceRetriever = &cwServiceRetriever{}
var s3ServiceRetriever IS3LogsServiceRetriever = &s3LogsServiceRetriever{}
var cloudWatchUploadFrequency = cloudwatchlogspublisher.UploadFrequency

type ICWServiceRetriever interface {
	NewCloudWatchLogsService(context context.T) ICloudWatchLoggingService
}

type IS3LogsServiceRetriever interface {
	NewAmazonS3Util(context context.T, outputS3BucketName string) (IS3Util, error)
}

type ICloudWatchLoggingService interface {
	StreamData(
		logGroupName string,
		logStreamName string,
		absoluteFilePath string,
		isFileComplete bool,
		isLogStreamCreated bool,
		fileCompleteSignal chan bool,
		cleanupControlCharacters bool,
		structuredLogs bool) (success bool)
	SetIsFileComplete(val bool)
	GetIsUploadComplete() bool
}

type IS3Util interface {
	S3Upload(logger log.T, outputS3BucketName string, s3Key string, filePath string) error
}

type cwServiceRetriever struct{}

func (cwServiceRetriever) NewCloudWatchLogsService(context context.T) ICloudWatchLoggingService {
	return cloudwatchlogspublisher.NewCloudWatchLogsService(context)
}

type s3LogsServiceRetriever struct{}

func (s3LogsServiceRetriever) NewAmazonS3Util(context context.T, outputS3BucketName string) (IS3Util, error) {
	return s3util.NewAmazonS3Util(context, outputS3BucketName)
}
