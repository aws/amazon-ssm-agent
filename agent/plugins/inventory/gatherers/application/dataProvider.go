package application

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
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
	maxSummaryLength    = 100
)

// decoupling package repository for easy testability
var packageRepository = localpackages.NewRepositoryDefault()

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

// CollectApplicationData collects all application data from the system using platform specific queries and merges in applications installed via configurePackage
func CollectApplicationData(context context.T) (appData []model.ApplicationData) {
	platformAppData := collectPlatformDependentApplicationData(context)
	packageAppData := packageRepository.GetInventoryData(context)

	//merge packageAppData into appData
	return model.MergeLists(platformAppData, packageAppData)
}

// cleanupJsonField converts a text to a json friendly text as follows:
// - converts multi-line fields to single line by removing all but the first line
// - escapes special characters
// - truncates remaining line to length no more than maxSummaryLength
func cleanupJsonField(field string) string {
	res := field
	endOfLinePos := strings.Index(res, "\n")
	if endOfLinePos >= 0 {
		res = res[0:endOfLinePos]
	}
	res = strings.Replace(res, `\`, `\\`, -1)
	res = strings.Replace(res, `"`, `\"`, -1)
	res = strings.Replace(res, "\t", `\t`, -1)
	if len(res) > maxSummaryLength {
		res = res[0:maxSummaryLength]
	}
	return res
}

// replaceMarkedFields finds substrings delimited by the start and end markers,
// removes the markers, and replaces the text between the markers with the result
// of calling the fieldReplacer function on that text substring. For example, if
// the input string is:  "a string with <a>text</a> marked"
// the startMarker is:   "<a>"
// the end marker is:    "</a>"
// and fieldReplacer is: strings.ToUpper
// then the output will be: "a string with TEXT marked"
func replaceMarkedFields(str, startMarker, endMarker string, fieldReplacer func(string) string) (newStr string, err error) {
	startIndex := strings.Index(str, startMarker)
	newStr = ""
	for startIndex >= 0 {
		newStr += str[:startIndex]
		fieldStart := str[startIndex+len(startMarker):]
		endIndex := strings.Index(fieldStart, endMarker)
		if endIndex < 0 {
			err = errors.New("Found startMarker without endMarker!")
			return
		}
		field := fieldStart[:endIndex]
		transformedField := fieldReplacer(field)
		newStr += transformedField
		str = fieldStart[endIndex+len(endMarker):]
		startIndex = strings.Index(str, startMarker)
	}
	newStr += str
	return newStr, nil
}
