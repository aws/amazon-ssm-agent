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

// Package processor implements MDS plugin processor
// processor_coreplugin contains the ICorePlugin implementation
package processor

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/processor"
	asocitscheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/carlescere/scheduler"
)

// Name returns the Plugin Name
func (p *Processor) Name() string {
	return name
}

// Execute starts the scheduling of the message processor plugin
func (p *Processor) Execute(context context.T) (err error) {

	log := p.context.Log()
	log.Infof("starting mdsprocessor polling")
	//process the older messages from Current & Pending folder
	p.processOlderMessages()

	if p.messagePollJob, err = scheduler.Every(pollMessageFrequencyMinutes).Minutes().Run(p.loop); err != nil {
		context.Log().Errorf("unable to schedule message processor. %v", err)
	}

	p.assocProcessor = processor.NewAssociationProcessor(context)
	if p.assocProcessor.PollJob, err = asocitscheduler.CreateScheduler(
		log,
		p.assocProcessor.ProcessAssociation,
		pollAssociationFrequencyMinutes); err != nil {
		context.Log().Errorf("unable to schedule association processor. %v", err)
	}
	return
}

// RequestStop handles the termination of the message processor plugin job
func (p *Processor) RequestStop(stopType contracts.StopType) (err error) {
	var waitTimeout time.Duration

	if stopType == contracts.StopTypeSoftStop {
		waitTimeout = time.Duration(p.context.AppConfig().Mds.StopTimeoutMillis) * time.Millisecond
	} else {
		waitTimeout = hardStopTimeout
	}

	var wg sync.WaitGroup

	// ask the message processor to stop
	p.stop()

	// shutdown the send command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.sendCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// shutdown the cancel command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.cancelCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// wait for everything to shutdown
	wg.Wait()
	return nil
}
