package application

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	amazonPublisherName = "amazon"
	amazonSsmAgentLinux = "amazon-ssm-agent"
	amazonSsmAgentWin   = "amazon ssm agent"
	awsToolsWindows     = "aws tools for windows"
	ec2ConfigService    = "ec2configservice"
	awsCfnBootstrap     = "aws-cfn-bootstrap"
	awsPVDrivers        = "aws pv drivers"
	awsAPIToolsPrefix   = "aws-apitools-"
	awsAMIToolsPrefix   = "aws-amitools-"
)

var selectAwsApps map[string]string

func init() {
	//NOTE:
	// For V1 - to filter out aws components from aws applications - we are using a list of all aws components that
	// have been identified in various OS - amazon linux, ubuntu, windows etc.
	// This is also useful for amazon linux ami - where all packages have Amazon.com as publisher.
	selectAwsApps = make(map[string]string)
	selectAwsApps[amazonSsmAgentLinux] = amazonPublisherName
	selectAwsApps[amazonSsmAgentWin] = amazonPublisherName
	selectAwsApps[awsToolsWindows] = amazonPublisherName
	selectAwsApps[ec2ConfigService] = amazonPublisherName
	selectAwsApps[awsCfnBootstrap] = amazonPublisherName
	selectAwsApps[awsPVDrivers] = amazonPublisherName
}

func componentType(applicationName string) model.ComponentType {
	formattedName := strings.TrimSpace(applicationName)
	formattedName = strings.ToLower(formattedName)

	var compType model.ComponentType

	//check if application is a known aws component or part of aws-apitool- or aws-amitools- tool set.
	if _, found := selectAwsApps[formattedName]; found || strings.Contains(formattedName, awsAPIToolsPrefix) || strings.Contains(formattedName, awsAMIToolsPrefix) {
		compType |= model.AWSComponent
	}

	return compType
}

func CollectApplicationData(context context.T) (appData []model.ApplicationData) {
	return collectPlatformDependentApplicationData(context)
}
