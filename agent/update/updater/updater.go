// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package main represents the entry point of the ssm agent updater.
package main

import (
	"flag"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/update/processor"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	legacyUpdaterArtifactsRoot      = "/var/log/amazon/ssm/update/"
	firstAgentWithNewUpdaterPath    = "1.1.86.0"
	defaultLogRootForUpdate         = "/var/log/amazon/ssm/"
	defaultLogFileName              = "amazon-ssm-agent-update.log"
	defaultWaitTimeForAgentToFinish = 2
)

var (
	log       logger.T
	updater   processor.T
	setRegion = platform.SetRegion
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
)

func init() {
	log = logger.GetUpdaterLogger(defaultLogRootForUpdate, defaultLogFileName)
	defer log.Flush()

	// Sleep 2 seconds to allow agent to finishing up it's work
	time.Sleep(defaultWaitTimeForAgentToFinish * time.Second)

	updater = processor.NewUpdater()

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
}

// Config holds Runtime info of plugins.
type Config struct {
	Instances map[string]string
}

func main() {
	defer log.Close()
	defer log.Flush()

	region, err := setRegion(log, "")
	if err != nil {
		log.Error("please specify the region to use.")
		return
	}
	log.Debug("Using region:", region)

	flag.Parse()

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
	detail := &processor.UpdateDetail{
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
	}

	if err := resolveUpdateDetail(detail); err != nil {
		log.Errorf(err.Error())
		return
	}

	log.Infof("Update root is: %v", detail.UpdateRoot)

	// Load UpdateContext from local storage, set current update with the new UpdateDetail
	context, err := updater.InitializeUpdate(log, detail)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	// Start or resume update
	if err = updater.StartOrResumeUpdate(log, context); err != nil {
		// Rolled back, but service cannot start, Update failed.
		updater.Failed(context, log, updateutil.ErrorUnexpected, err.Error(), false)
	} else {
		log.Infof(context.Current.StandardOut)
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

	compareResult, err = updateutil.VersionCompare(detail.SourceVersion, firstAgentWithNewUpdaterPath)
	if err != nil {
		return err
	}
	// New versions that with new binary locations
	if compareResult >= 0 {
		detail.UpdateRoot = appconfig.UpdaterArtifactsRoot
	} else {
		detail.UpdateRoot = legacyUpdaterArtifactsRoot
	}

	return nil
}
