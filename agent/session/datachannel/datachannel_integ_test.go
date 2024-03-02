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

// Package datachannel implements data channel which is used to interactively run commands.
package datachannel

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

	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	wsUpgrader = &websocket.Upgrader{ReadBufferSize: 2048, WriteBufferSize: 2084}
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func httpHandler(hw http.ResponseWriter, request *http.Request) {
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

func TestOpenDataChannel_MultiThread(suite *testing.T) {
	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	dataChannel := getDataChannelRef()
	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	dataChannel.Initialize(mockContext, mockService, sessionId, clientId, instanceId, mgsConfig.RolePublishSubscribe, mockCancelFlag, inputStreamMessageHandler)
	var err error

	// Set local server URL
	dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler)
	dataChannel.wsChannel.SetUrl(u.String())
	assert.Nil(suite, err, "should not throw error during websocket creation")

	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	// start the data channel Open
	dataChannel.Open(mockContext.Log())
	defer dataChannel.Close(mockLog)
	assert.Nil(suite, err, "should not throw error during channel open")

	// sleep for 1 minute to wait for the goroutines to run
	time.Sleep(20 * time.Second)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(suite, initialGRNumber+5 >= completedGRNumber)
}

func TestOpenDataChannel_OpenDataChannelError(suite *testing.T) {
	httpErrorHandler := func(hw http.ResponseWriter, request *http.Request) {
		http.Error(hw, fmt.Sprintf("no upgrade: %v", fmt.Errorf("test")), http.StatusGatewayTimeout)
		return
	}

	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpErrorHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	dataChannel := getDataChannelRef()
	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	dataChannel.Initialize(mockContext, mockService, sessionId, clientId, instanceId, mgsConfig.RolePublishSubscribe, mockCancelFlag, inputStreamMessageHandler)
	var err error

	err = dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler)
	dataChannel.wsChannel.SetUrl(u.String())
	assert.Nil(suite, err, "should not throw error during websocket creation")

	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	// start the control channel Open
	err = dataChannel.Open(mockContext.Log())
	defer dataChannel.Close(mockLog)
	assert.NotNil(suite, err, "should throw error during channel open")

	completedGRNumber := runtime.NumGoroutine()
	fmt.Println(initialGRNumber)
	fmt.Println(completedGRNumber)
	// Adding buffer as tests runs parallely
	assert.True(suite, initialGRNumber+5 >= completedGRNumber)
}

func TestOpenDataChannel_CreateDataChannelError_RetryCount(t *testing.T) {
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

	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, fmt.Errorf("test"))
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	counter := 0
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	startTime := time.Now()

	// copied over from sessionplugin
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			counter++
			dataChannel := getDataChannelRef()
			dataChannel.wsChannel.SetUrl(u.String())
			if err := dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler); err != nil {
				return nil, fmt.Errorf("failed to create websocket for datachannel with error: %s", err)
			}

			if err := dataChannel.Open(mockLog); err != nil {
				return nil, fmt.Errorf("failed to open datachannel with error: %s", err)
			}
			dataChannel.ResendStreamDataMessageScheduler(mockLog)
			return dataChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.DataChannelRetryInitialDelayMillis) + mgsConfig.DataChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.DataChannelNumMaxAttempts,
	}

	retryer.Init()
	_, err1 := retryer.Call()

	assert.NotNil(t, err1)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, math.Abs(startTime.Sub(time.Now()).Seconds()) < 10)
	time.Sleep(20 * time.Second)
	// make sure that only 2 additional goroutines should be running
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
	assert.Equal(t, 6, counter)
}

func TestOpenDataChannel_OpenDataChannelError_RetryCount(t *testing.T) {
	httpTempHandler := func(hw http.ResponseWriter, request *http.Request) {
		http.Error(hw, fmt.Sprintf("no upgrade: %v", fmt.Errorf("test")), http.StatusGatewayTimeout)
	}
	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpTempHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	counter := 0
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	startTime := time.Now()

	// copied over from sessionplugin
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			counter++
			dataChannel := getDataChannelRef()

			if err := dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler); err != nil {
				return nil, fmt.Errorf("failed to create websocket for datachannel with error: %s", err)
			}
			dataChannel.wsChannel.SetUrl(u.String())
			if err := dataChannel.Open(mockLog); err != nil {
				return nil, fmt.Errorf("failed to open datachannel with error: %s", err)
			}
			dataChannel.ResendStreamDataMessageScheduler(mockLog)
			return dataChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.DataChannelRetryInitialDelayMillis) + mgsConfig.DataChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.DataChannelNumMaxAttempts,
	}

	retryer.Init()
	_, err1 := retryer.Call()

	assert.NotNil(t, err1)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, math.Abs(startTime.Sub(time.Now()).Seconds()) < 10)
	time.Sleep(20 * time.Second)
	// make sure that only 2 additional goroutines should be running
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
	assert.Equal(t, 6, counter)
}

func TestOpenDataChannel_OpenDataChannelError_RetryCount_AfterConnection(t *testing.T) {
	// launch local HTTP Server
	srv := httptest.NewServer(http.HandlerFunc(httpHandler))
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	defer srv.Close()

	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)

	counter := 0
	// Get number of go-routines running
	initialGRNumber := runtime.NumGoroutine()

	startTime := time.Now()

	// copied over from sessionplugin
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			counter++
			dataChannel := getDataChannelRef()
			dataChannel.wsChannel.SetUrl(u.String())
			if err := dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler); err != nil {
				return nil, fmt.Errorf("failed to create websocket for datachannel with error: %s", err)
			}

			if err := dataChannel.Open(mockLog); err != nil {
				return nil, fmt.Errorf("failed to open datachannel with error: %s", err)
			}
			dataChannel.ResendStreamDataMessageScheduler(mockLog)
			return dataChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.DataChannelRetryInitialDelayMillis) + mgsConfig.DataChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.DataChannelNumMaxAttempts,
	}

	retryer.Init()
	_, err1 := retryer.Call()

	assert.NotNil(t, err1)
	completedGRNumber := runtime.NumGoroutine()
	assert.True(t, math.Abs(startTime.Sub(time.Now()).Seconds()) < 10)
	time.Sleep(20 * time.Second)
	// make sure that only 2 additional goroutines should be running
	assert.True(t, initialGRNumber+5 >= completedGRNumber)
	assert.Equal(t, 6, counter)
}

func getDataChannelRef() *DataChannel {
	dataChannel := &DataChannel{}
	dataChannel.Initialize(mockContext,
		mockService,
		sessionId,
		clientId,
		instanceId,
		mgsConfig.RolePublishSubscribe,
		mockCancelFlag,
		inputStreamMessageHandler)
	return dataChannel
}
