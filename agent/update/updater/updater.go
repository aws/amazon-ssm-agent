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
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/nightlyone/lockfile"
)

const (
	defaultLogFileName              = "AmazonSSMAgent-update.txt"
	defaultWaitTimeForAgentToFinish = 3
)

var (
	updater      processor.T
	log          logger.T
	agentContext context.T
)

var (
	update           *bool
	sourceVersion    *string
	sourceLocation   *string
	sourceHash       *string
	targetVersion    *string
	targetLocation   *string
	targetHash       *string
	packageName      *string
	messageID        *string
	stdout           *string
	stderr           *string
	outputKeyPrefix  *string
	outputBucket     *string
	manifestURL      *string
	selfUpdate       *bool
	disableDowngrade *bool
)

var newAgentIdentity = identity.NewAgentIdentity

func init() {
	log = ssmlog.GetUpdaterLogger(logger.DefaultLogDir, defaultLogFileName)

	// Load update detail from command line
	update = flag.Bool(updateconstants.UpdateCmd, false, "current Agent Version")
	sourceVersion = flag.String(updateconstants.SourceVersionCmd, "", "current Agent Version")
	sourceLocation = flag.String(updateconstants.SourceLocationCmd, "", "current Agent installer source")

	targetVersion = flag.String(updateconstants.TargetVersionCmd, "", "target Agent Version")

	packageName = flag.String(updateconstants.PackageNameCmd, "", "target Agent Version")
	messageID = flag.String(updateconstants.MessageIDCmd, "", "target Agent Version")
	stdout = flag.String(updateconstants.StdoutFileName, "", "standard output file path")
	stderr = flag.String(updateconstants.StderrFileName, "", "standard error file path")
	outputKeyPrefix = flag.String(updateconstants.OutputKeyPrefixCmd, "", "output key prefix")
	outputBucket = flag.String(updateconstants.OutputBucketNameCmd, "", "output bucket name")

	manifestURL = flag.String(updateconstants.ManifestFileUrlCmd, "", "Manifest file url")

	selfUpdate = flag.Bool(updateconstants.SelfUpdateCmd, false, "SelfUpdate command")

	disableDowngrade = flag.Bool(updateconstants.DisableDowngradeCmd, false, "defines if updater is allowed to downgrade")

	// Legacy flags no longer used, need to be defined or we get this error: flag provided but not defined
	flag.String(updateconstants.TargetLocationCmd, "", "target Agent installer source")
	flag.String(updateconstants.TargetHashCmd, "", "target Agent installer hash")
	flag.String(updateconstants.SourceHashCmd, "", "current Agent installer hash")
}

func main() {
	defer log.Close()
	defer log.Flush()

	log.Infof("SSM Agent Updater - %s", version.String())
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

	// Create update info
	updateInfo, err := updateinfo.New(agentContext)
	if err != nil {
		log.Errorf("Failed to initialize update info object: %v", err)
		os.Exit(1)
	}

	// Sleep 3 seconds to allow agent to finishing up it's work
	time.Sleep(defaultWaitTimeForAgentToFinish * time.Second)

	updater = processor.NewUpdater(agentContext, updateInfo)

	// If the updater already owns the lockfile, no harm done
	// If there is no lockfile, the updater will own it
	// If the updater is unable to lock the file, we retry and then fail
	lock, _ := lockfile.New(appconfig.UpdaterPidLockfile)
	err = lock.TryLockExpireWithRetry(updateconstants.UpdateLockFileMinutes)

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

	// Return if update is not present in the command
	if !*update {
		log.Error("incorrect usage (use -update).")
		flag.Usage()
		return
	}

	// Basic Validation
	if len(*manifestURL) == 0 && len(*sourceLocation) == 0 {
		log.Error("must pass either manifest url or source location")
		flag.Usage()
	}
	if len(*sourceVersion) == 0 {
		log.Error("no current version")
		flag.Usage()
	}
	if !*selfUpdate && len(*targetVersion) == 0 {
		log.Error("no target version")
		flag.Usage()
	}

	// Create new UpdateDetail
	updateDetail := &processor.UpdateDetail{
		State:              processor.NotStarted,
		Result:             contracts.ResultStatusInProgress,
		SourceVersion:      *sourceVersion,
		SourceLocation:     *sourceLocation,
		TargetVersion:      *targetVersion,
		StdoutFileName:     *stdout,
		StderrFileName:     *stderr,
		OutputS3KeyPrefix:  *outputKeyPrefix,
		OutputS3BucketName: *outputBucket,
		PackageName:        *packageName,
		MessageID:          *messageID,
		StartDateTime:      time.Now().UTC(),
		RequiresUninstall:  false,
		ManifestURL:        *manifestURL,
		Manifest:           updatemanifest.New(agentContext, updateInfo),
		SelfUpdate:         *selfUpdate,
		AllowDowngrade:     !*disableDowngrade,
	}

	updateDetail.UpdateRoot, err = resolveUpdateRoot(updateDetail.SourceVersion)
	if err != nil {
		log.Errorf("Failed to resolve update root: %v", err)
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
		updater.Failed(updateDetail, log, updateconstants.ErrorUnexpected, err.Error(), false)
	} else {
		log.Infof(updateDetail.StandardOut)
	}

}

// recoverUpdaterFromPanic recovers updater if panic occurs and fails the updater
func recoverUpdaterFromPanic(updateDetail *processor.UpdateDetail) {
	// recover in case the updater panics
	if err := recover(); err != nil {
		agentContext.Log().Errorf("recovered from panic for updater %v!", err)
		agentContext.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		updater.Failed(updateDetail, agentContext.Log(), updateconstants.ErrorUnexpectedThroughPanic, fmt.Sprintf("%v", err), false)
	}
}
