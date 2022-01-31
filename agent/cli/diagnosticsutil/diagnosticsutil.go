// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package diagnosticsutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	agentContext "github.com/aws/amazon-ssm-agent/agent/context"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	DiagnosticsStatusSuccess = "Success"
	DiagnosticsStatusSkipped = "Skipped"
	DiagnosticsStatusFailed  = "Failed"
)

type DiagnosticOutput struct {
	Check  string
	Status string
	Note   string
}

type DiagnosticQuery interface {
	Execute() DiagnosticOutput
	GetName() string
	GetPriority() int
}

var DiagnosticQueries []DiagnosticQuery

// RegisterDiagnosticQuery registers a new diagnostics query to be used by get-diagnostics
func RegisterDiagnosticQuery(diagnosticQuery DiagnosticQuery) {
	DiagnosticQueries = append(DiagnosticQueries, diagnosticQuery)
}

// GetAwsSession create a single session and shares the session cross diagnostics queries
func GetAwsSession(agentIdentity identity.IAgentIdentity, service string) (*session.Session, error) {
	awsConfig := sdkutil.AwsConfig(agentContext.Default(logger.NewSilentMockLog(), appconfig.DefaultConfig(), agentIdentity), service)
	return session.NewSession(awsConfig)
}

func IsOnPremRegistration() bool {
	// Only wait for 2 seconds, if instance is onprem this is instant
	isOnPremChan := make(chan bool, 1)
	go func() {
		agentIdentity, identityErr := cliutil.GetAgentIdentity()

		isOnPremChan <- identityErr == nil && identity.IsOnPremInstance(agentIdentity)
	}()

	select {
	case onPremBool := <-isOnPremChan:
		return onPremBool
	case <-time.After(2 * time.Second):
		return false
	}
}

func GetSSMAgentVersion() (string, error) {
	agentPath, err := getAgentFilePath()
	if err != nil {
		return "", err
	}

	output, err := ExecuteCommandWithTimeout(time.Second, agentPath, "-version")

	if err != nil {
		return "", err
	}

	versionSplit := strings.Split(output, ": ")
	if len(versionSplit) != 2 {
		return "", fmt.Errorf("Unexpected result from version flag: '%s'", output)
	}

	return versionSplit[1], nil
}

func ExecuteCommandWithTimeout(timeout time.Duration, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	byteArr, err := exec.CommandContext(ctx, cmd, args...).Output()
	output := strings.TrimSpace(string(byteArr))

	return output, err
}

func IsAgentInstalledSnap() bool {
	_, err := ExecuteCommandWithTimeout(2*time.Second, "snap", "services", "amazon-ssm-agent")
	return err == nil
}

func getRunningAgentPid() (int, error) {
	processQuerier := executor.NewProcessExecutor(logger.NewSilentMockLog())

	procs, err := processQuerier.Processes()

	if err != nil {
		return -1, err
	}

	ssmAgentBinaryPath, err := getAgentProcessPath()
	if err != nil {
		return -1, err
	}

	for _, proc := range procs {
		if proc.Executable == ssmAgentBinaryPath {
			return proc.Pid, nil
		}
	}

	return -1, fmt.Errorf("agent process not found")
}

// Column order in table formatter
var columnOrder = []string{
	"Check",
	"Status",
	"Note",
}

// Symbols
const (
	topLeftCorner  = "┌"
	topRightCorner = "┐"
	botLeftCorner  = "└"
	botRightCorner = "┘"

	horizontalDash = "─"
	verticalDash   = "│"

	botT   = "┴"
	topT   = "┬"
	leftT  = "├"
	rightT = "┤"
	plus   = "┼"
)

type TableFormatter struct {
	maxWidth    int
	tableWidth  int
	jsonMap     []map[string]string
	columnWidth map[string]int
	columnMap   map[string]string
}

// padRight appends a single character to a str until it has reached the desired length
func (t *TableFormatter) padRight(str, pad string, length int) string {
	if len(str) >= length {
		return str
	}

	return str + strings.Repeat(pad, length-len(str))
}

func (t *TableFormatter) init() {
	t.columnWidth = map[string]int{}
	lastColIndex := len(columnOrder) - 1
	lastCol := columnOrder[lastColIndex]
	for _, execution := range t.jsonMap {
		for _, column := range columnOrder {
			strLen := len(execution[column])
			if strLen > t.columnWidth[column] {
				t.columnWidth[column] = strLen
			}
		}
	}

	// Calculate width of last column
	totalWidthExcludingLastCol := 0
	totalWidth := 0
	for col, width := range t.columnWidth {
		if col != lastCol {
			totalWidthExcludingLastCol += width
		}
		totalWidth += width
	}

	// Spacing is 2 on each side plus 3 between each column (num col - 1)
	spacing := 4 + (3 * (lastColIndex))
	if totalWidth+spacing > t.maxWidth {
		t.columnWidth[lastCol] = t.maxWidth - spacing - totalWidthExcludingLastCol

	}

	// get min width of last column
	largestWord := 0
	for _, execution := range t.jsonMap {
		for _, word := range strings.Split(execution[lastCol], " ") {
			if len(word) > largestWord {
				largestWord = len(word)
			}
		}
	}

	// if allowed width is less than largest word, set width as largest word
	if t.columnWidth[lastCol] < largestWord {
		t.columnWidth[lastCol] = largestWord
	}

	// set total table width
	t.tableWidth = spacing
	for _, width := range t.columnWidth {
		t.tableWidth += width
	}

	// initialize column map
	t.columnMap = map[string]string{}
	for _, col := range columnOrder {
		t.columnMap[col] = col
	}
}

func (t *TableFormatter) createSplitRow(b *strings.Builder, leftSymbol, rightSymbol, columnSplitSymbol string) {
	for i := 0; i < len(columnOrder); i++ {
		if i == 0 {
			b.WriteString(leftSymbol + t.padRight("", horizontalDash, t.columnWidth[columnOrder[i]]+2))
		} else if i+1 == len(columnOrder) {
			b.WriteString(columnSplitSymbol + t.padRight("", horizontalDash, t.columnWidth[columnOrder[i]]+2) + rightSymbol + newlineCharacter)
		} else {
			b.WriteString(columnSplitSymbol + t.padRight("", horizontalDash, t.columnWidth[columnOrder[i]]+2))
		}
	}
}

func (t *TableFormatter) createTextRow(b *strings.Builder, row map[string]string) {
	for i := 0; i < len(columnOrder); i++ {
		b.WriteString(verticalDash + " " + t.padRight(row[columnOrder[i]], " ", t.columnWidth[columnOrder[i]]) + " ")
	}
	b.WriteString(verticalDash + newlineCharacter)
}

func (t *TableFormatter) splitRow(row map[string]string) (result []map[string]string) {
	lastColIndex := len(columnOrder) - 1
	lastCol := columnOrder[lastColIndex]
	if len(row[lastCol]) <= t.columnWidth[lastCol] {
		return []map[string]string{
			row,
		}
	}

	b := &strings.Builder{}

	// Cleanup note strings
	note := strings.ReplaceAll(row[lastCol], "\n", " ")
	note = strings.ReplaceAll(note, "\t", " ")
	note = strings.ReplaceAll(note, "  ", " ")
	wordList := strings.Split(note, " ")

	for i := 0; i < len(wordList); i++ {
		word := wordList[i]
		if b.Len()+len(word) > t.columnWidth[lastCol] {
			if len(result) == 0 {
				row[lastCol] = b.String()
				row[lastCol] = row[lastCol][:len(row[lastCol])-1]
				result = append(result, row)
			} else {
				entryMap := map[string]string{}
				for _, col := range columnOrder {
					entryMap[col] = ""
				}

				text := b.String()
				entryMap[lastCol] = text[:len(text)-1]
				result = append(result, entryMap)
			}

			b.Reset()
		}

		b.WriteString(word + " ")
	}

	if b.Len() > 0 {
		entryMap := map[string]string{}
		for _, col := range columnOrder {
			entryMap[col] = ""
		}

		text := b.String()
		entryMap[lastCol] = text[:len(text)-1]
		result = append(result, entryMap)
	}

	return result
}

func (t *TableFormatter) String() string {
	t.init()

	b := &strings.Builder{}
	// Create header
	t.createSplitRow(b, topLeftCorner, topRightCorner, topT)
	t.createTextRow(b, t.columnMap)

	for _, execution := range t.jsonMap {
		t.createSplitRow(b, leftT, rightT, plus)
		rows := t.splitRow(execution)
		for _, row := range rows {
			t.createTextRow(b, row)
		}
	}

	t.createSplitRow(b, botLeftCorner, botRightCorner, botT)

	return b.String()
}

func NewTableFormatter(maxWidth int, executions []DiagnosticOutput) TableFormatter {
	// Convert array to json
	jsonMap := make([]map[string]string, len(executions))
	jsonString, _ := json.Marshal(executions)
	_ = json.Unmarshal(jsonString, &jsonMap)

	return TableFormatter{
		maxWidth: maxWidth,
		jsonMap:  jsonMap,
	}
}
