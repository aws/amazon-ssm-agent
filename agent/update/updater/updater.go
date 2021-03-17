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

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/update/processor"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/nightlyone/lockfile"
)

const (
	defaultLogFileName              = "AmazonSSMAgent-update.txt"
	defaultWaitTimeForAgentToFinish = 3
	defaultStdoutFileName           = "stdout"
	defaultStderrFileName           = "stderr"
	defaultSSMAgentName             = "amazon-ssm-agent"
	defaultSelfUpdateMessageID      = "aws.ssm.self-update-agent.i-instanceid"
)

var (
	updater      processor.T
	log          logger.T
	agentContext context.T
)

var (
	update          *bool
	sourceVersion   *string
	sourceLocation  *string
	sourceHash      *string
	targetVersion   *string
	targetLocation  *string
	targetHash      *string
	packageName     *string
	messageID       *string
	stdout          *string
	stderr          *string
	outputKeyPrefix *string
	outputBucket    *string
	manifestURL     *string
	manifestPath    *string
	selfUpdate      *bool
)

var newAgentIdentity = identity.NewAgentIdentity

func init() {
	log = ssmlog.GetUpdaterLogger(logger.DefaultLogDir, defaultLogFileName)

	// Load update detail from command line
	update = flag.Bool(updateutil.UpdateCmd, false, "current Agent Version")
	sourceVersion = flag.String(updateutil.SourceVersionCmd, "", "current Agent Version")
	sourceLocation = flag.String(updateutil.SourceLocationCmd, "", "current Agent installer source")
	sourceHash = flag.String(updateutil.SourceHashCmd, "", "current Agent installer hash")
	targetVersion = flag.String(updateutil.TargetVersionCmd, "", "target Agent Version")
	targetLocation = flag.String(updateutil.TargetLocationCmd, "", "target Agent installer source")
	targetHash = flag.String(updateutil.TargetHashCmd, "", "target Agent installer hash")
	packageName = flag.String(updateutil.PackageNameCmd, "", "target Agent Version")
	messageID = flag.String(updateutil.MessageIDCmd, "", "target Agent Version")
	stdout = flag.String(updateutil.StdoutFileName, "", "standard output file path")
	stderr = flag.String(updateutil.StderrFileName, "", "standard error file path")
	outputKeyPrefix = flag.String(updateutil.OutputKeyPrefixCmd, "", "output key prefix")
	outputBucket = flag.String(updateutil.OutputBucketNameCmd, "", "output bucket name")

	manifestURL = flag.String(updateutil.ManifestFileUrlCmd, "", "Manifest file url")
	manifestLocation := ""
	manifestPath = &manifestLocation

	selfUpdate = flag.Bool(updateutil.SelfUpdateCmd, false, "SelfUpdate command")

}

func main() {
	defer log.Close()
	defer log.Flush()

	// Initialize agent config for agent identity
	appConfig, err := appconfig.Config(true)
	if err != nil {
		log.Warnf("Failed to load agent config: %v", err)
	}
	// Create identity selector
	selector := identity.NewDefaultAgentIdentitySelector(log)
	agentIdentity, err := newAgentIdentity(log, &appConfig, selector)
	if err != nil {
		log.Errorf("Failed to assume agent identity: %v", err)
		os.Exit(1)
	}

	agentContext = context.Default(log, appConfig, agentIdentity)

	// Sleep 3 seconds to allow agent to finishing up it's work
	time.Sleep(defaultWaitTimeForAgentToFinish * time.Second)

	updater = processor.NewUpdater(agentContext)

	// If the updater already owns the lockfile, no harm done
	// If there is no lockfile, the updater will own it
	// If the updater is unable to lock the file, we retry and then fail
	lock, _ := lockfile.New(appconfig.UpdaterPidLockfile)
	err = lock.TryLockExpireWithRetry(updateutil.UpdateLockFileMinutes)

	if err != nil {
		if err == lockfile.ErrBusy {
			log.Warnf("Failed to lock update lockfile, another update is in progress: %s", err)
			return
		} else {
			log.Warnf("Proceeding update process with new lock. Failed to lock update lockfile: %s", err)
		}
	}

	defer lock.Unlock()

	flag.Parse()

	// self update command,
	if *selfUpdate {
		// manifest path will only be specified in self update use case
		var err error
		if len(*manifestURL) == 0 {
			log.Error("Please provide manifest path for self update")
			flag.Usage()
		}

		log.Infof("Starting getting self update required information")

		if *sourceLocation, *sourceHash, *targetVersion, *targetLocation, *targetHash, *manifestURL, *manifestPath, err =
			updateutil.PrepareResourceForSelfUpdate(agentContext, *manifestURL, *sourceVersion); err != nil {
			log.Errorf(err.Error())
			return
		}

		log.WriteEvent(logger.AgentUpdateResultMessage,
			*sourceVersion,
			updateutil.GenerateSelUpdateSuccessEvent(string(updateutil.Stage))) // UpdateSucceeded_SelfUpdate_Stage

		if *targetVersion == *sourceVersion {
			log.Infof("Current version %v is not deprecated, skipping self update", *sourceVersion)
			return
		}

		*stdout = defaultStdoutFileName
		*stderr = defaultStderrFileName
		*packageName = defaultSSMAgentName
		*messageID = defaultSelfUpdateMessageID

		log.Infof("stdout: %v", *stdout)
		log.Infof("stderr: %v", *stderr)
		log.Infof("packageName: %v", *packageName)
		log.Infof("messageId: %v", *messageID)

		// current version and current resource download url location
		log.Infof("sourceVersion : %v", *sourceVersion)
		log.Infof("sourceLocation : %v", *sourceLocation)
		log.Infof("sourceHash : %v", *sourceHash)

		// latest active version and resource download url location
		log.Infof("targetVersion : %v", *targetVersion)
		log.Infof("targetLocation : %v", *targetLocation)
		log.Infof("targetHash : %v", *targetHash)
	}

	// Return if update is not present in the command
	if !*update {
		log.Error("incorrect usage (use -update).")
		flag.Usage()
		return
	}

	// Basic Validation
	if len(*sourceVersion) == 0 || len(*sourceLocation) == 0 {
		log.Error("no current version or package source.")
		flag.Usage()
	}
	if len(*targetVersion) == 0 || len(*targetLocation) == 0 {
		log.Error("no target version or package source.")
		flag.Usage()
	}

	// Create new UpdateDetail
	updateDetail := &processor.UpdateDetail{
		State:              processor.NotStarted,
		Result:             contracts.ResultStatusInProgress,
		SourceVersion:      *sourceVersion,
		SourceLocation:     *sourceLocation,
		SourceHash:         *sourceHash,
		TargetVersion:      *targetVersion,
		TargetLocation:     *targetLocation,
		TargetHash:         *targetHash,
		StdoutFileName:     *stdout,
		StderrFileName:     *stderr,
		OutputS3KeyPrefix:  *outputKeyPrefix,
		OutputS3BucketName: *outputBucket,
		PackageName:        *packageName,
		MessageID:          *messageID,
		StartDateTime:      time.Now().UTC(),
		RequiresUninstall:  false,
		ManifestUrl:        *manifestURL,
		ManifestPath:       *manifestPath,
		SelfUpdate:         *selfUpdate,
	}

	if err = resolveUpdateDetail(updateDetail); err != nil {
		log.Errorf(err.Error())
		return
	}

	log.Infof("Update root is: %v", updateDetail.UpdateRoot)

	// Initialize update detail with plugin info
	err = updater.InitializeUpdate(log, updateDetail)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	// Recover updater if panic occurs and fail the updater
	defer recoverUpdaterFromPanic(updateDetail)

	// Start or resume update
	if err = updater.StartOrResumeUpdate(log, updateDetail); err != nil { // We do not send any error above this to ICS/MGS except panic message
		// Rolled back, but service cannot start, Update failed.
		updater.Failed(updateDetail, log, updateutil.ErrorUnexpected, err.Error(), false)
	} else {
		log.Infof(updateDetail.StandardOut)
	}

}

// resolveUpdateDetail decides which UpdaterRoot to use and if uninstall is required for the agent
func resolveUpdateDetail(detail *processor.UpdateDetail) error {
	compareResult, err := updateutil.VersionCompare(detail.SourceVersion, detail.TargetVersion)
	if err != nil {
		return err
	}
	// if performing a downgrade
	if compareResult > 0 {
		detail.RequiresUninstall = true
	}

	if err := updateRoot(detail); err != nil {
		return err
	}

	return nil
}

// recoverUpdaterFromPanic recovers updater if panic occurs and fails the updater
func recoverUpdaterFromPanic(updateDetail *processor.UpdateDetail) {
	// recover in case the updater panics
	if err := recover(); err != nil {
		agentContext.Log().Errorf("recovered from panic for updater %v!", err)
		agentContext.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		updater.Failed(updateDetail, agentContext.Log(), updateutil.ErrorUnexpectedThroughPanic, fmt.Sprintf("%v", err), false)
	}
}
