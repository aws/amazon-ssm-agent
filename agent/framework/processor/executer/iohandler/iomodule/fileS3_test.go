package iomodule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFileS3Read(t *testing.T) {
	var TestInputCases = [...]string{
		"Test input text.",
		"A sample \ninput text.",
		"\b5Ὂg̀9! ℃ᾭG",
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non. " +
			"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. " +
			"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, " +
			"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat" +
			" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer" +
			" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, " +
			"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae " +
			"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu " +
			"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
	}

	i := 0

	for _, testCase := range TestInputCases {
		file := File{
			FileName:               "file" + strconv.Itoa(i),
			OrchestrationDirectory: "testdata",
			OutputS3BucketName:     "s3Bucket",
			OutputS3KeyPrefix:      "s3KeyPrefix",
			LogGroupName:           "LogGroup",
			LogStreamName:          "LogStream",
		}
		outputFileExists := testFileS3Read(appconfig.DefaultPluginOutputRetention, testCase, file)
		assert.True(t, outputFileExists)
		i++
	}

}

func TestFileS3CleansUpAfterS3Upload(t *testing.T) {
	file := File{
		FileName:               "TestFileS3CleansUpAfterS3Upload",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "bucket-to-upload-to",
		OutputS3KeyPrefix:      "s3KeyPrefix",
		LogGroupName:           "",
		LogStreamName:          "",
	}

	outputFileExists := testFileS3Read(appconfig.PluginLocalOutputCleanupAfterUpload, "Test input text.", file)
	assert.False(t, outputFileExists)

}

func TestFileS3DoesntCleanUpAfterS3Upload(t *testing.T) {
	file := File{
		FileName:               "TestFileS3DoesntCleanUpAfterS3Upload",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "bucket-to-upload-to",
		OutputS3KeyPrefix:      "s3KeyPrefix",
		LogGroupName:           "",
		LogStreamName:          "",
	}

	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	var mockS3Util = &s3UtilMock{}
	config := appconfig.SsmagentConfig{}
	config.Ssm.PluginLocalOutputCleanup = appconfig.DefaultPluginOutputRetention
	var context = contextmocks.NewMockDefaultWithConfig(config)
	s3Key := fileutil.BuildS3Path(file.OutputS3KeyPrefix, file.FileName)
	filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)
	mockS3Util.On("S3Upload", mock.AnythingOfType("*log.Mock"), file.OutputS3BucketName, s3Key, filePath).Return(fmt.Errorf("upload error"))

	var s3RetrieverMock = &s3LogsServiceRetrieverMock{}
	s3RetrieverMock.On("NewAmazonS3Util", mock.AnythingOfType("*context.Mock"), file.OutputS3BucketName).Return(mockS3Util, nil)
	s3ServiceRetriever = s3RetrieverMock

	var mockCWLoggingService = &cloudWatchLoggingServiceMock{}

	var cwRetrieverMock = &cloudWatchServiceRetrieverMock{}
	cwRetrieverMock.On("NewCloudWatchLogsService", mock.AnythingOfType("*context.Mock")).Return(mockCWLoggingService)
	cloudWatchServiceRetriever = cwRetrieverMock

	wg.Add(1)

	go func() {
		defer wg.Done()
		file.Read(context, r, appconfig.SuccessExitCode)
	}()

	w.Write([]byte("Test input text."))
	w.Close()
	wg.Wait()
	outputFileExists, _ := fileutil.LocalFileExist(filePath)
	if outputFileExists {
		os.Remove(filePath)
	}

	assert.True(t, outputFileExists)
}

func TestFileS3DoesntCleanUpAfterRebootExitCode(t *testing.T) {
	file := File{
		FileName:               "TestFileS3DoesntCleanUpAfterS3Upload",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "bucket-to-upload-to",
		OutputS3KeyPrefix:      "s3KeyPrefix",
		LogGroupName:           "",
		LogStreamName:          "",
	}

	config := appconfig.SsmagentConfig{}
	config.Ssm.PluginLocalOutputCleanup = appconfig.DefaultPluginOutputRetention
	var context = contextmocks.NewMockDefaultWithConfig(config)

	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	var mockS3Util = &s3UtilMock{}
	s3Key := fileutil.BuildS3Path(file.OutputS3KeyPrefix, file.FileName)
	filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)
	mockS3Util.On("S3Upload", mock.AnythingOfType("*log.Mock"), file.OutputS3BucketName, s3Key, filePath).Return(nil)

	var s3RetrieverMock = &s3LogsServiceRetrieverMock{}
	s3RetrieverMock.On("NewAmazonS3Util", mock.AnythingOfType("*context.Mock"), file.OutputS3BucketName).Return(mockS3Util, nil)
	s3ServiceRetriever = s3RetrieverMock

	var mockCWLoggingService = &cloudWatchLoggingServiceMock{}

	var cwRetrieverMock = &cloudWatchServiceRetrieverMock{}
	cwRetrieverMock.On("NewCloudWatchLogsService", mock.AnythingOfType("*context.Mock")).Return(mockCWLoggingService)
	cloudWatchServiceRetriever = cwRetrieverMock

	wg.Add(1)

	go func() {
		defer wg.Done()
		file.Read(context, r, appconfig.RebootExitCode)
	}()

	w.Write([]byte("Test input text."))
	w.Close()
	wg.Wait()
	outputFileExists, _ := fileutil.LocalFileExist(filePath)
	if outputFileExists {
		os.Remove(filePath)
	}

	assert.True(t, outputFileExists)
}

func TestFileS3CleanUpAfterCloudWatchUpload(t *testing.T) {
	file := File{
		FileName:               "TestFileS3CleanUpAfterS3CloudwatchUpload",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "",
		OutputS3KeyPrefix:      "",
		LogGroupName:           "log-group",
		LogStreamName:          "log-stream",
	}

	outputFileExists := testFileS3Read(appconfig.PluginLocalOutputCleanupAfterUpload, "Test input text.", file)
	assert.False(t, outputFileExists)
}

func TestFileS3DoesntCleanUpAfterS3CloudwatchUpload(t *testing.T) {
	config := appconfig.SsmagentConfig{}
	config.Ssm.PluginLocalOutputCleanup = appconfig.PluginLocalOutputCleanupAfterUpload
	var context = contextmocks.NewMockDefaultWithConfig(config)

	file := File{
		FileName:               "TestFileS3CleanUpAfterS3CloudwatchUpload",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "",
		OutputS3KeyPrefix:      "",
		LogGroupName:           "log-group",
		LogStreamName:          "log-stream",
	}

	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)

	var mockCWLoggingService = &cloudWatchLoggingServiceMock{}
	mockCWLoggingService.On("StreamData",
		file.LogGroupName,
		file.LogStreamName,
		filePath,
		false,
		false,
		mock.AnythingOfType("chan bool"),
		false,
		false).Return(true)

	mockCWLoggingService.On("SetIsFileComplete", mock.AnythingOfType("bool")).Return()
	mockCWLoggingService.On("GetIsUploadComplete").Return(false).Times(62)

	var cwRetrieverMock = &cloudWatchServiceRetrieverMock{}
	cwRetrieverMock.On("NewCloudWatchLogsService", mock.AnythingOfType("*context.Mock")).Return(mockCWLoggingService)
	cloudWatchServiceRetriever = cwRetrieverMock

	wg.Add(1)

	go func() {
		defer wg.Done()
		file.Read(context, r, appconfig.SuccessExitCode)
	}()

	w.Write([]byte("Test input text."))
	w.Close()
	wg.Wait()
	outputFileExists, _ := fileutil.LocalFileExist(filePath)
	if outputFileExists {
		os.Remove(filePath)
	}

	assert.True(t, outputFileExists)
}

func TestFileS3CleansUpAfterExecution(t *testing.T) {
	file := File{
		FileName:               "TestFileS3CleansUpAfterExecution",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "",
		OutputS3KeyPrefix:      "",
		LogGroupName:           "",
		LogStreamName:          "",
	}

	outputFileExists := testFileS3Read(appconfig.PluginLocalOutputCleanupAfterExecution, "Test input text.", file)

	assert.False(t, outputFileExists)
}

func TestFileS3DefaultPluginOutputRetention(t *testing.T) {
	file := File{
		FileName:               "TestFileS3DefaultPluginOutputRetention",
		OrchestrationDirectory: "testdata",
		OutputS3BucketName:     "s3Bucket",
		OutputS3KeyPrefix:      "s3KeyPrefix",
		LogGroupName:           "LogGroup",
		LogStreamName:          "LogStream",
	}

	outputFileExists := testFileS3Read(appconfig.DefaultPluginOutputRetention, "Test input text.", file)

	assert.True(t, outputFileExists)
}

func testFileS3Read(pluginLocalOutputCleanupPref string, pipeTestCase string, file File) bool {
	config := appconfig.SsmagentConfig{}
	config.Ssm.PluginLocalOutputCleanup = pluginLocalOutputCleanupPref
	var context = contextmocks.NewMockDefaultWithConfig(config)

	r, w := io.Pipe()
	wg := new(sync.WaitGroup)
	var mockS3Util = &s3UtilMock{}
	s3Key := fileutil.BuildS3Path(file.OutputS3KeyPrefix, file.FileName)
	filePath := filepath.Join(file.OrchestrationDirectory, file.FileName)
	mockS3Util.On("S3Upload", mock.AnythingOfType("*log.Mock"), file.OutputS3BucketName, s3Key, filePath).Return(nil)

	var s3RetrieverMock = &s3LogsServiceRetrieverMock{}
	s3RetrieverMock.On("NewAmazonS3Util", mock.AnythingOfType("*context.Mock"), file.OutputS3BucketName).Return(mockS3Util, nil)
	s3ServiceRetriever = s3RetrieverMock

	var mockCWLoggingService = &cloudWatchLoggingServiceMock{}
	mockCWLoggingService.On("StreamData",
		file.LogGroupName,
		file.LogStreamName,
		filePath,
		false,
		false,
		mock.AnythingOfType("chan bool"),
		false,
		false).Return(true)

	mockCWLoggingService.On("SetIsFileComplete", mock.AnythingOfType("bool")).Return()
	mockCWLoggingService.On("GetIsUploadComplete").Return(true).Times(2)

	var cwRetrieverMock = &cloudWatchServiceRetrieverMock{}
	cwRetrieverMock.On("NewCloudWatchLogsService", mock.AnythingOfType("*context.Mock")).Return(mockCWLoggingService)
	cloudWatchServiceRetriever = cwRetrieverMock

	wg.Add(1)

	go func() {
		defer wg.Done()
		file.Read(context, r, appconfig.SuccessExitCode)
	}()

	w.Write([]byte(pipeTestCase))
	w.Close()
	wg.Wait()
	outputFileExists, _ := fileutil.LocalFileExist(filePath)
	if outputFileExists {
		os.Remove(filePath)
	}

	return outputFileExists
}
