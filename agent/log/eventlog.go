// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package log is used to initialize the logger(main logger and event logger). This package should be imported once, usually from main, then call GetLogger.
package log

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
)

var (
	eventLogInst       *EventLog
	singleSpacePattern = regexp.MustCompile(`\s+`)
)

//GetEventLog returns the Event log instance and is called by the SSM Logger during app startup
func GetEventLog(logFilePath string, logFileName string) (eventLog *EventLog) {
	if eventLogInst != nil {
		return eventLogInst
	}
	var maxRollsDay int = appconfig.DefaultAuditExpirationDay
	config, err := appconfig.Config(true)
	if err == nil {
		maxRollsDay = config.Agent.AuditExpirationDay
	}
	eventLogInstance := EventLog{
		eventChannel:     make(chan string, 2),
		noOfHistoryFiles: maxRollsDay,
		schemaVersion:    "1",
		eventLogPath:     filepath.Join(logFilePath, "audits"),
		eventLogName:     logFileName,
		datePattern:      "2006-01-02",
		fileSystem:       filesystem.NewFileSystem(),
		timePattern:      "15:04:05", // HH:MM:SS
	}
	eventLogInstance.init()
	eventLogInstance.rotateEventLog()
	eventLogInst = &eventLogInstance
	return eventLogInst
}

// Creates events with the appropriate file parameters passed to this instance
type EventLog struct {
	eventChannel     chan string // Used for passing events to file write go routine.
	noOfHistoryFiles int         // Number of audit files to maintain in log folder
	eventLogPath     string      // Log file path
	eventLogName     string      // Event Log Name
	schemaVersion    string      // Schema version
	datePattern      string      // Date Pattern used for creating files
	fileSystem       filesystem.IFileSystem
	timePattern      string

	currentFileName string // Name of File currently being used for logging in this instance. On app startup, it will be empty
	nextFileName    string // Current day's log file name
	fileDelimiter   string
}

//Init sets the Default value for instance
func (e *EventLog) init() {
	e.currentFileName = ""
	e.fileDelimiter = "-"
	e.nextFileName = e.eventLogName + e.fileDelimiter + time.Now().Format(e.datePattern)
	if err := e.fileSystem.MkdirAll(e.eventLogPath, appconfig.ReadWriteExecuteAccess); err != nil {
		fmt.Println("Failed to create directory for audits", err)
	}
	e.eventWriter()
}

//Getters

// GetTodayAuditFileName will return the audit file name of currently used one
func (e *EventLog) GetTodayAuditFileName() string {
	return e.nextFileName
}

// GetAuditFileName will return the audit file name without the date pattern
func (e *EventLog) GetAuditFileName() string {
	return e.eventLogName
}

//GetAuditFilePath will return the audit file path
func (e *EventLog) GetAuditFilePath() string {
	return e.eventLogPath
}

//GetAuditFilePath will return the file system instance
func (e *EventLog) GetFileSystem() filesystem.IFileSystem {
	return e.fileSystem
}

//loadEvent loads the event to the channel to be passed to the write file go routine
func (e *EventLog) loadEvent(eventType string, agentVersion string, eventContent string) {
	// Time appended to the message in the format HH:MM:SS
	if agentVersion == "" {
		agentVersion = version.Version
	}
	eventContent = eventType + " " + eventContent + " " + agentVersion + " " + time.Now().Format(e.timePattern) + "\n"
	e.eventChannel <- eventContent
}

//close closes the buffered channel
func (e *EventLog) close() {
	close(e.eventChannel)
}

//rotateEventLog checks for the deletion of files and deleted it
func (e *EventLog) rotateEventLog() {
	validFileNames := e.getFilesWithMatchDatePattern()
	deleteFileCount := len(validFileNames) - e.noOfHistoryFiles
	for i := 0; i < deleteFileCount; i++ {
		logFilePathWithDate := filepath.Join(e.eventLogPath, validFileNames[i])
		e.fileSystem.DeleteFile(logFilePathWithDate)
	}
}

//eventWriter triggers the go routine once and then waits for data from buffer channel
func (e *EventLog) eventWriter() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Event writer panic: ", r)
			}
		}()
		for event := range e.eventChannel {
			header := SchemaVersionHeader + e.schemaVersion + "\n"
			if createdFlag := e.writeFile(event, header); createdFlag {
				e.rotateEventLog()
			}
		}
	}()
}

//getFilesWithMatchDatePattern gets the files matching with the date pattern
func (e *EventLog) getFilesWithMatchDatePattern() []string {
	var validFileNames []string
	if allFiles, err := e.fileSystem.ReadDir(e.eventLogPath); err == nil {
		for _, fileInfo := range allFiles {
			fileName := fileInfo.Name()
			if !fileInfo.Mode().IsDir() && e.isValidFileName(fileName) {
				validFileNames = append(validFileNames, fileName)
			}
		}
	}
	return validFileNames
}

//isValidFileName checks whether the file matches the Date pattern
func (e *EventLog) isValidFileName(fileName string) bool {
	logFileWithDelim := e.eventLogName + e.fileDelimiter
	if !strings.HasPrefix(fileName, logFileWithDelim) {
		return false
	}
	datePart := fileName[len(logFileWithDelim):]
	_, err := time.ParseInLocation(e.datePattern, datePart, time.Local)
	if err != nil {
		return false
	}
	return true
}

// writeFile writes events and header to the file.
// When the file is not available, Creates a new file and inserts the header
// When the file is available, updates the file
func (e *EventLog) writeFile(content string, header string) (createdFlag bool) {
	logFilePathWithDate := filepath.Join(e.eventLogPath, e.nextFileName)
	if !e.currentDateFileExists() {
		createdFlag = true
		content = header + content
	}
	if err := e.fileSystem.AppendToFile(logFilePathWithDate, content, appconfig.ReadWriteAccess); err != nil {
		fmt.Println("Failed to write on the event log.", err)
		return
	}
	e.currentFileName = e.nextFileName
	return
}

// currentDateFileExists checks whether the current day file exists
func (e *EventLog) currentDateFileExists() bool {
	if e.currentFileName == "" {
		if _, err := e.fileSystem.Stat(filepath.Join(e.eventLogPath, e.nextFileName)); e.fileSystem.IsNotExist(err) {
			return false
		}
		return true
	}
	return e.currentFileName == e.nextFileName
}

// The below functions uses the eventlog singleton instance and use only the old audit logs to work on.

// EventCounter contains the audit count, file name and the audit date to be sent to MGS
type EventCounter struct {
	AuditFileName  string         // audit file name
	AuditDate      string         // Can be used later. Date to which the audit file belongs to.
	CountMap       map[string]int // count of events found in audit log
	SchemaVersion  string         // schema version from audit file
	AgentVersion   string         // denotes agent version found in the audit log
	LastReadTime   string         // denotes last read time stamp from file
	LastReadByte   int            // denotes last read byte from file
	IsFirstChunk   bool           // denotes first chunk taken from file
	EventChunkType string         // denotes message type used to send to MGS
}

// createUpdateEventCounter creates and updates event counter object based on the event line and time marker. This function creates new object when the version is new.
func createUpdateEventCounter(eventCounterObj *EventCounter, eventLine string, byteMarker int) (*EventCounter, bool) {
	var eventChunkType string
	eventLine = singleSpacePattern.ReplaceAllString(strings.TrimSpace(eventLine), " ")
	eventSplitVal := strings.Split(eventLine, " ")

	// For Invalid Data (skips lines with less than 4 words)
	if len(eventSplitVal) < 4 {
		eventCounterObj.LastReadByte = byteMarker
		return eventCounterObj, false
	}

	eventChunkType, eventName, version, timeStamp := eventSplitVal[0], eventSplitVal[1], eventSplitVal[2], eventSplitVal[3]
	if matched, err := regexp.MatchString(VersionRegexPattern, version); matched == false || err != nil {
		eventCounterObj.LastReadByte = byteMarker
		return eventCounterObj, false
	}
	// Will create a new object and load data for it from new chunk. For now, the chunks are divided based on version and Update events.
	newlyCreated := false
	if eventCounterObj.AgentVersion != "" && (version != eventCounterObj.AgentVersion ||
		eventChunkType != eventCounterObj.EventChunkType ||
		eventChunkType == AgentUpdateResultMessage) {
		newlyCreated = true
		eventCounterObj = &EventCounter{
			AuditFileName: eventCounterObj.AuditFileName,
			AuditDate:     eventCounterObj.AuditDate,
			CountMap:      make(map[string]int),
			SchemaVersion: eventCounterObj.SchemaVersion,
			IsFirstChunk:  false,
		}
	}
	eventCounterObj.EventChunkType = eventChunkType
	eventCounterObj.AgentVersion = version
	eventCounterObj.CountMap[eventName]++
	eventCounterObj.LastReadTime = timeStamp
	eventCounterObj.LastReadByte = byteMarker
	return eventCounterObj, newlyCreated
}

// WriteLastLineFile updates the file name with the Audit success status. This should be locked by the caller if called by multiple threads.
func WriteLastLineFile(eventCounter *EventCounter) error {
	if eventLogInst == nil {
		return nil
	}

	// generates byte marker with padding zeros
	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(BytePatternLen)+"d", eventCounter.LastReadByte)
	logfilePath := filepath.Join(eventLogInst.eventLogPath, eventCounter.AuditFileName)

	// Creates footer with last read byte padded by zeros
	if eventCounter.IsFirstChunk {
		// Appends the footer
		if err := eventLogInst.fileSystem.AppendToFile(logfilePath, AuditSentSuccessFooter+byteMarker, appconfig.ReadWriteAccess); err != nil {
			return err
		}
		return nil
	}
	// Updates footer of Audit file with last read byte padded by zeros
	stat, err := os.Stat(logfilePath)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(logfilePath, os.O_RDWR, appconfig.ReadWriteAccess)
	if err != nil {
		return err
	}
	defer file.Close()
	lastReadByteBegin := stat.Size() - BytePatternLen
	_, err = file.WriteAt([]byte(byteMarker), lastReadByteBegin)
	if err != nil {
		return err
	}
	return nil
}

// GetEventCounter returns the count of the audits in the previous days logs.
// Returns empty list when an exception is thrown by file handlers
func GetEventCounter() ([]*EventCounter, error) {
	eventCounters := make([]*EventCounter, 0)
	if eventLogInst == nil {
		return eventCounters, nil
	}
	nextFileName := eventLogInst.eventLogName + eventLogInst.fileDelimiter + time.Now().Format(eventLogInst.datePattern)
	validFileNames := eventLogInst.getFilesWithMatchDatePattern()
	// Loop continues till it visits the file with Audit Sent log
	validFileNamesLen := len(validFileNames) - 1
	for idx := validFileNamesLen; idx >= 0 && idx >= validFileNamesLen-2; idx-- { // Considers only last two files and ignores today's file
		if validFileNames[idx] == nextFileName {
			continue
		}
		auditLogFileName := filepath.Join(eventLogInst.eventLogPath, validFileNames[idx])
		isAuditFileProcessed, byteMarker, err := isAuditSentToMGS(auditLogFileName)
		if err != nil || byteMarker < 0 { // byte marker is set to -1 when the file has been processed
			continue
		}
		eventCounterObj, err := countEvent(auditLogFileName, byteMarker, isAuditFileProcessed)
		if err != nil {
			continue
		}
		eventCounters = append(eventCounters, eventCounterObj...)
	}
	return eventCounters, nil
}

// countEvent returns the count of the audits for the file passed and stores event greater than the time marker
func countEvent(fileName string, byteMarker int, isAuditFileProcessed bool) ([]*EventCounter, error) {
	// reads header
	eventCounterObj, offset, err := readEventLogHeaders(fileName)
	if err != nil {
		return nil, err
	}
	eventCounterObj.IsFirstChunk = !isAuditFileProcessed // denotes that the file is untouched. Not even a single section is sent

	// sets the offset value to be forwarded in file.
	// retrieved value from footer of processed file
	if byteMarker > 0 {
		offset = byteMarker
	}

	// reads footer
	eventCounter, err := readEventLogBodyFooter(fileName, eventCounterObj, offset)
	if err != nil {
		return nil, err
	}
	return eventCounter, nil
}

// readHeaders reads body from audit file and returns the event counter object loaded with header information
func readEventLogBodyFooter(fileName string, eventCounterObj *EventCounter, offset int) ([]*EventCounter, error) {
	eventCounter := make([]*EventCounter, 0)
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// seek offset bytes from beginning of file
	file.Seek(int64(offset), 0)

	// creates a new scanner with custom split function
	scanner := bufio.NewScanner(file)
	scanner.Split(splitAuditLine(&offset))

	for scanner.Scan() {
		// For the footer
		if strings.HasPrefix(scanner.Text(), AuditSentSuccessFooter) {
			break
		}
		//For the data part
		if newObj, created := createUpdateEventCounter(eventCounterObj, scanner.Text(), offset); created { // TODO when file grows pass line number to createUpdateEventCounter and break
			if len(eventCounter) == 4 { // Reads only 4 + 1 ( Added after for loop) chunks from the file
				break
			}
			eventCounter = append(eventCounter, eventCounterObj)
			eventCounterObj = newObj
		}
	}
	eventCounter = append(eventCounter, eventCounterObj)
	// Reverse array - For now, two elem with two versions
	for i, j := 0, len(eventCounter)-1; i < j; i, j = i+1, j-1 {
		eventCounter[i], eventCounter[j] = eventCounter[j], eventCounter[i]
	}
	return eventCounter, nil
}

// readHeaders reads headers from audit file and returns the event counter object loaded with header information
func readEventLogHeaders(fileName string) (*EventCounter, int, error) {
	noOfBytesRead := 0
	file, err := os.Open(fileName)
	if err != nil {
		return nil, noOfBytesRead, err
	}
	defer file.Close()

	eventCounterObj := &EventCounter{
		CountMap:      make(map[string]int),
		AuditFileName: filepath.Base(fileName),
		AgentVersion:  "",
	}
	filePrefixLen := len(eventLogInst.eventLogName + eventLogInst.fileDelimiter)
	eventCounterObj.AuditDate = eventCounterObj.AuditFileName[filePrefixLen:]

	scanner := bufio.NewScanner(file)
	scanner.Split(splitAuditLine(&noOfBytesRead))

	skipLineCount := 1
	for scanner.Scan() {
		// For the Header
		if skipLineCount > 0 {
			skipLineCount--
			eventCounterObj.SchemaVersion = getValidAuditFileHeaderFooterVal(scanner.Text(), SchemaVersionHeader) // Can be iterated when the headers are more
		}
		if skipLineCount == 0 {
			break
		}
	}
	return eventCounterObj, noOfBytesRead, nil
}

// splitAuditLine helps in splitting event log using default bufio.ScanLines and retrieves number of bytes read
func splitAuditLine(byteLen *int) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanLines(data, atEOF)
		*byteLen += advance
		return advance, token, err
	}
}

// getValidAuditFileHeaderFooterVal returns valid(current when the field blank) header and footer val from the audit file
func getValidAuditFileHeaderFooterVal(event string, headerFooterTag string) string {
	val := getEventHeaderVal(event, headerFooterTag)
	switch {
	case strings.HasPrefix(event, SchemaVersionHeader):
		if val == "" {
			val = eventLogInst.schemaVersion // Current schema version
		}
	}
	return val
}

// getEventHeaderVal returns the value from header and footer
func getEventHeaderVal(event string, headerFooterTag string) string {
	if strings.HasPrefix(event, headerFooterTag) {
		headerSplitVal := strings.Split(event, headerFooterTag)
		if len(headerSplitVal) > 1 {
			return headerSplitVal[1]
		}
	}
	return ""
}

// isAuditSentToMGS checks whether AuditSentSuccess status is present or not and returns the last audit sent time if present
func isAuditSentToMGS(auditFileName string) (bool, int, error) {
	byteMarker := 0
	stat, err := os.Stat(auditFileName)
	if err != nil {
		return false, byteMarker, err
	}
	file, err := os.Open(auditFileName)
	if err != nil {
		return false, byteMarker, err
	}
	defer file.Close()
	buf := make([]byte, len(AuditSentSuccessFooter)+BytePatternLen)
	start := stat.Size() - int64(len(buf))
	_, err = file.ReadAt(buf, start)
	if err != nil {
		return false, byteMarker, err
	}
	lastLine := string(buf)
	if strings.HasPrefix(lastLine, AuditSentSuccessFooter) {
		lastLineSplitVal := strings.Split(lastLine, AuditSentSuccessFooter)
		if len(lastLineSplitVal) > 1 {
			byteMarker, _ = strconv.Atoi(lastLineSplitVal[1])
			if byteMarker < 0 { // Using int instead of uint here for avoiding extra conversions
				byteMarker = 0
			}
			// when the file is processed completely
			if int64(byteMarker) == start {
				return true, -1, nil
			}
		}
		return true, byteMarker, nil
	}
	return false, byteMarker, nil
}
