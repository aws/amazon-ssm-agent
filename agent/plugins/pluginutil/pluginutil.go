// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package pluginutil implements some common functions shared by multiple plugins.
package pluginutil

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	defaultExecutionTimeoutInSeconds = 3600
	maxExecutionTimeoutInSeconds     = 172800
	minExecutionTimeoutInSeconds     = 5
)

// StringPrefix returns the beginning part of a string, truncated to the given limit.
func StringPrefix(input string, maxLength int, truncatedSuffix string) string {
	// no need to truncate
	if len(input) < maxLength {
		return input
	}

	// truncate and add suffix
	if maxLength > len(truncatedSuffix) {
		pos := maxLength - len(truncatedSuffix)
		return string(input[:pos]) + truncatedSuffix
	}

	// suffix longer than maxLength - return beginning of suffix
	return truncatedSuffix[:maxLength]
}

// ReadPrefix returns the beginning data from a given Reader, truncated to the given limit.
func ReadPrefix(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
	// read up to maxLength bytes from input
	data, err := ioutil.ReadAll(io.LimitReader(input, int64(maxLength)))
	if err != nil {
		return
	}

	out = StringPrefix(string(data), maxLength, truncatedSuffix)
	return
}

// ReadAll returns all data from a given Reader.
func ReadAll(input io.Reader, maxLength int, truncatedSuffix string) (out string, err error) {
	// read up to maxLength bytes from input
	data, err := ioutil.ReadAll(io.LimitReader(input, int64(maxLength)))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// CreateScriptFile creates a script containing the given commands.
func CreateScriptFile(log log.T, scriptPath string, runCommand []string, byteOrderMark fileutil.ByteOrderMark) (err error) {
	// write source commands to file
	_, err = fileutil.WriteIntoFileWithPermissionsExtended(scriptPath, strings.Join(runCommand, "\n")+"\n", appconfig.ReadWriteExecuteAccess, byteOrderMark)
	if err != nil {
		log.Errorf("failed to write runcommand scripts to file %v, err %v", scriptPath, err)
		return
	}

	return
}

// DownloadFileFromSource downloads file from source
func DownloadFileFromSource(context context.T, source string, sourceHash string, sourceHashType string) (artifact.DownloadOutput, error) {
	// download source and verify its integrity
	downloadInput := artifact.DownloadInput{
		SourceURL:            source,
		DestinationDirectory: appconfig.DownloadRoot,
		SourceChecksums: map[string]string{
			sourceHashType: sourceHash,
		},
	}
	context.Log().Debug("Downloading file")
	return artifact.Download(context, downloadInput)
}

// LoadParametersAsList returns properties as a list and appropriate PluginResult if error is encountered
func LoadParametersAsList(log log.T, prop interface{}, res *contracts.PluginResult) (properties []interface{}) {

	switch prop := prop.(type) {
	case []interface{}:
		if err := jsonutil.Remarshal(prop, &properties); err != nil {
			log.Errorf("unable to parse plugin configuration")
			res.Output = "Execution failed because agent is unable to parse plugin configuration"
			res.Code = 1
			res.Status = contracts.ResultStatusFailed
		}
	default:
		properties = append(properties, prop)
	}
	return
}

// LoadParametersAsMap returns properties as a map and appropriate PluginResult if error is encountered
func LoadParametersAsMap(log log.T, prop interface{}, out iohandler.IOHandler) (properties map[string]interface{}) {

	if err := jsonutil.Remarshal(prop, &properties); err != nil {
		log.Errorf("unable to parse plugin configuration")
		out.AppendError("Execution failed because agent is unable to parse plugin configuration")
		out.SetExitCode(1)
		out.SetStatus(contracts.ResultStatusFailed)
	}
	return
}

// ValidateExecutionTimeout validates the supplied input interface and converts it into a valid int value.
func ValidateExecutionTimeout(log log.T, input interface{}) int {
	var num int

	switch input.(type) {
	case string:
		num = extractIntFromString(log, input.(string))
	case int:
		num = input.(int)
	case float64:
		f := input.(float64)
		num = int(f)
		log.Infof("Unexpected 'TimeoutSeconds' float value %v received. Applying 'TimeoutSeconds' as %v", f, num)
	default:
		log.Infof("Unexpected 'TimeoutSeconds' value %v received. Setting 'TimeoutSeconds' to default value %v", input, defaultExecutionTimeoutInSeconds)
	}

	if num < minExecutionTimeoutInSeconds || num > maxExecutionTimeoutInSeconds {
		log.Infof("'TimeoutSeconds' value should be between %v and %v. Setting 'TimeoutSeconds' to default value %v", minExecutionTimeoutInSeconds, maxExecutionTimeoutInSeconds, defaultExecutionTimeoutInSeconds)
		num = defaultExecutionTimeoutInSeconds
	}
	return num
}

// ParseRunCommand checks the command type and convert it to the string array
func ParseRunCommand(input interface{}, output []string) []string {
	switch value := input.(type) {
	case string:
		output = append(output, value)
	case []interface{}:
		for _, element := range value {
			output = ParseRunCommand(element, output)
		}
	}
	return output
}

// extractIntFromString extracts a valid int value from a string.
func extractIntFromString(log log.T, input string) int {
	var iNum int
	var fNum float64
	var err error

	iNum, err = strconv.Atoi(input)
	if err == nil {
		return iNum
	}

	fNum, err = strconv.ParseFloat(input, 64)
	if err == nil {
		iNum = int(fNum)
		log.Infof("Unexpected 'TimeoutSeconds' float value %v received. Applying 'TimeoutSeconds' as %v", fNum, iNum)
	} else {
		log.Errorf("Unexpected 'TimeoutSeconds' string value %v received. Setting 'TimeoutSeconds' to default value %v", input, defaultExecutionTimeoutInSeconds)
		iNum = defaultExecutionTimeoutInSeconds
	}
	return iNum
}

// GetProxySetting returns proxy setting from registry entries
func GetProxySetting(proxyValue []string) (string, string) {
	var url string
	var noProxy string

	for _, proxy := range proxyValue {
		tmp := strings.TrimSpace(proxy)
		parts := strings.Split(tmp, "=")
		switch parts[0] {
		case "http_proxy":
			url = parts[1]
		case "no_proxy":
			noProxy = parts[1]
		}
	}

	return url, noProxy
}

// ReplaceMarkedFields finds substrings delimited by the start and end markers,
// removes the markers, and replaces the text between the markers with the result
// of calling the fieldReplacer function on that text substring. For example, if
// the input string is:  "a string with <a>text</a> marked"
// the startMarker is:   "<a>"
// the end marker is:    "</a>"
// and fieldReplacer is: strings.ToUpper
// then the output will be: "a string with TEXT marked"
func ReplaceMarkedFields(str, startMarker, endMarker string, fieldReplacer func(string) string) (newStr string, err error) {
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

// CleanupNewLines removes all newlines from the given input
func CleanupNewLines(s string) string {
	return strings.Replace(strings.Replace(s, "\n", "", -1), "\r", "", -1)
}

// CleanupJSONField converts a text to a json friendly text as follows:
// - converts multi-line fields to single line by removing all but the first line
// - escapes special characters
// - truncates remaining line to length no more than maxSummaryLength
func CleanupJSONField(field string) string {
	res := field
	endOfLinePos := strings.Index(res, "\n")
	if endOfLinePos >= 0 {
		res = res[0:endOfLinePos]
	}
	res = strings.Replace(res, `\`, `\\`, -1)
	res = strings.Replace(res, `"`, `\"`, -1)
	res = strings.Replace(res, "\t", `\t`, -1)
	return res
}

// Deletes file if it exists
func CleanupFile(log log.T, file string) {
	if _, err := os.Stat(file); err == nil || os.IsExist(err) {
		if err = os.RemoveAll(file); err != nil {
			log.Debugf("failed to delete file %v, %v", file, err.Error())
		} else {
			log.Debugf("deleted file %v", file)
		}
	} else {
		log.Debugf("failed to get file info: %v", file)
	}
}
