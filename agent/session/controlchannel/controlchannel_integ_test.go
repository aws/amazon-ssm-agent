// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build integration
// +build integration

// controlchannel package implement control communicator for web socket connection.
package controlchannel

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/session/service/mocks"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	wsUpgrader = &websocket.Upgrader{ReadBufferSize: 2048, WriteBufferSize: 2084}
)

func TestMain(m *testing.M) {
	resetConnectionChannel()
	code := m.Run()
	os.Exit(code)
}

func TestOpenControlChannel_MultiThread(t *testing.T) {
	httpErrorHandler := func(hw http.ResponseWriter, request *http.Request) {
		httpConn, err := wsUpgrader.Upgrade(hw, request, nil)
		if err != nil {
			http.Error(hw, fmt.Sprintf("no upgrade: %v", err), http.StatusGatewayTimeout)
			panic("Connection should be successful. Should not enter here.")
		}

		for {
			_, _, err = httpConn.ReadMessage()
			if err != nil {
				return
			}
			// close connection to simulate any issue on service side
			httpConn.Close()
		}
	}

	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpErrorHandler))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	controlChannel := getControlChannel()
	messageChan := make(chan mgsContracts.AgentMessage)
	mockEventLog.On("SendAuditMessage")
	var ableToOpenMGSConnection uint32
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}
	mockService = &serviceMock.Service{}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	controlChannel.Initialize(mockContext, mockService, instanceId, messageChan)
	var err error
	err = controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection)
	assert.Nil(t, err, "should not throw error during websocket creation")

	// Set local server URL
	controlChannel.wsChannel.SetUrl(u.String())

	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()
	stop := make(chan bool)
	startConnectionChannelReader(stop, contracts.MGS)
	// start the control channel Open
	err = controlChannel.Open(mockContext, &ableToOpenMGSConnection)
	defer controlChannel.Close(mockLog)

	// sleep for 1 minute to wait for the goroutines to run
	time.Sleep(60 * time.Second)
	assert.Nil(t, err, "should not throw error during channel open")

	controlChannel.AuditLogScheduler.ScheduleAuditEvents()
	stop <- true

	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, initialGRNumber+5 >= completedGRNumber) // tests run in parallel at times hence adding some buffer
}

func startConnectionChannelReader(stop chan bool, expectedStatus contracts.SSMConnectionChannel) {
	go func() {
		for {
			select {
			case status := <-ssmconnectionchannel.GetMDSSwitchChannel():
				if expectedStatus == contracts.MDS {
					if status {
						break
					}
					panic("should not reach this spot for MGS")
				}
				if expectedStatus == contracts.MGS {
					if !status {
						break
					}
					panic("should not reach this spot for MGS")
				}
				break
			case <-stop:
				return
			}
		}
	}()
}

func TestOpenControlChannel_OpenControlChannelError(t *testing.T) {
	httpErrorHandler := func(hw http.ResponseWriter, request *http.Request) {
		http.Error(hw, fmt.Sprintf("no upgrade: %v", fmt.Errorf("test")), http.StatusGatewayTimeout)
		return
	}

	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpErrorHandler))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"

	controlChannel := getControlChannel()
	messageChan := make(chan mgsContracts.AgentMessage)
	mockEventLog.On("SendAuditMessage")
	var ableToOpenMGSConnection uint32
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}
	mockService = &serviceMock.Service{}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	stop := make(chan bool)
	startConnectionChannelReader(stop, contracts.MDS)

	controlChannel.Initialize(mockContext, mockService, instanceId, messageChan)
	var err error
	err = controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection)
	assert.Nil(t, err, "should not throw error during websocket creation")

	// Set local server URL
	controlChannel.wsChannel.SetUrl(u.String())
	defer controlChannel.wsChannel.Close(mockContext.Log())

	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	// start control channel Open
	err = controlChannel.Open(mockContext, &ableToOpenMGSConnection)

	defer controlChannel.Close(mockLog)
	assert.NotNil(t, err, "should throw error during channel open")

	time.Sleep(10 * time.Second)
	completedGRNumber := runtime.NumGoroutine()
	stop <- true

	// tests run in parallel at times hence adding some buffer
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
}

func TestOpenControlChannel_CreateControlChannelError(t *testing.T) {
	controlChannel := getControlChannel()
	messageChan := make(chan mgsContracts.AgentMessage)
	mockEventLog.On("SendAuditMessage")
	var ableToOpenMGSConnection uint32
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}
	mockService = &serviceMock.Service{}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, fmt.Errorf("throw error"))
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)
	stop := make(chan bool)
	startConnectionChannelReader(stop, contracts.MGS)
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()
	controlChannel.Initialize(mockContext, mockService, instanceId, messageChan)
	var err error
	err = controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection)
	defer controlChannel.Close(mockLog)
	assert.Contains(t, err.Error(), "throw error")
	time.Sleep(2 * time.Second)
	completedGRNumber := runtime.NumGoroutine()
	stop <- true
	// tests run in parallel at times hence adding some buffer
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
}

func TestOpenControlChannel_CreateControlChannelError_RetryCount(t *testing.T) {
	httpTempHandler := func(hw http.ResponseWriter, request *http.Request) {
		httpConn, err := wsUpgrader.Upgrade(hw, request, nil)
		if err != nil {
			http.Error(hw, fmt.Sprintf("no upgrade: %v", err), http.StatusGatewayTimeout)
			panic("Connection should be successful. Should not enter here.")
		}
		for {
			_, _, err = httpConn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpTempHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	controlChannel := getControlChannel()
	messageChan := make(chan mgsContracts.AgentMessage)

	// Set local server URL
	mockEventLog.On("SendAuditMessage")

	var ableToOpenMGSConnection uint32
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}

	mockService = &serviceMock.Service{}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, fmt.Errorf("test")).Times(3)
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	counter := 0
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	startTime := time.Now()

	stop := make(chan bool)
	startConnectionChannelReader(stop, contracts.MGS)

	// copied over from MGSInteractor
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			counter++
			controlChannel = getControlChannel()
			controlChannel.Initialize(mockContext, mockService, instanceId, messageChan)
			if err = controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection); err != nil {
				return nil, err
			}

			controlChannel.wsChannel.SetUrl(u.String())
			if err = controlChannel.Open(mockContext, &ableToOpenMGSConnection); err != nil {
				return nil, err
			}

			controlChannel.AuditLogScheduler.ScheduleAuditEvents()
			return controlChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		JitterRatio:         mgsConfig.RetryJitterRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.ControlChannelRetryInitialDelayMillis) + mgsConfig.ControlChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.ControlChannelRetryMaxIntervalMillis,
		MaxAttempts:         30,
	}
	retryer.Init()
	_, err1 := retryer.Call()

	stop <- true
	assert.Nil(t, err1)
	time.Sleep(10 * time.Second)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, math.Abs(startTime.Sub(time.Now()).Seconds()) > 50)

	// tests run in parallel at times hence adding some buffer
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
	assert.Equal(t, 4, counter)
}

func TestOpenControlChannel_OpenControlChannelError_RetryCount(t *testing.T) {
	httpTempHandler := func(hw http.ResponseWriter, request *http.Request) {
		http.Error(hw, fmt.Sprintf("no upgrade: %v", fmt.Errorf("err1")), http.StatusGatewayTimeout)
	}
	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpTempHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	controlChannel := getControlChannel()
	messageChan := make(chan mgsContracts.AgentMessage)

	// Set local server URL
	mockEventLog.On("SendAuditMessage")

	var ableToOpenMGSConnection uint32
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}

	mockService = &serviceMock.Service{}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, nil).Times(4)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	counter := 0
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	startTime := time.Now()
	stop := make(chan bool)
	startConnectionChannelReader(stop, contracts.MDS)
	// copied over from MGSInteractor
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			counter++
			controlChannel = getControlChannel()
			controlChannel.Initialize(mockContext, mockService, instanceId, messageChan)
			if err = controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection); err != nil {
				return nil, err
			}

			controlChannel.wsChannel.SetUrl(u.String())
			if err = controlChannel.Open(mockContext, &ableToOpenMGSConnection); err != nil {
				return nil, err
			}

			controlChannel.AuditLogScheduler.ScheduleAuditEvents()
			return controlChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		JitterRatio:         mgsConfig.RetryJitterRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.ControlChannelRetryInitialDelayMillis) + mgsConfig.ControlChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.ControlChannelRetryMaxIntervalMillis,
		MaxAttempts:         3,
	}
	retryer.Init()
	_, err1 := retryer.Call()

	assert.NotNil(t, err1)
	stop <- true
	time.Sleep(10 * time.Second)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, math.Abs(startTime.Sub(time.Now()).Seconds()) > 50)

	// tests run in parallel at times hence adding some buffer
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
	assert.Equal(t, 4, counter)
	mockService.AssertExpectations(t)
}
