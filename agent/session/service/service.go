// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package service is a wrapper for the message gateway Service
package service

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/rolecreds"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	mgsconfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

const (
	mgsClientTimeout      = time.Second * 15
	httpStatusCodeCreated = 201
)

// Service is an interface to the message gateway service operation v1.
type Service interface {
	CreateControlChannel(log log.T, createControlChannelInput *CreateControlChannelInput, channelId string) (createControlChannelOutput *CreateControlChannelOutput, err error)
	CreateDataChannel(log log.T, createDataChannelInput *CreateDataChannelInput, sessionId string) (createDataChannelOutput *CreateDataChannelOutput, err error)
	GetV4Signer() *v4.Signer
	GetRegion() string
}

// MessageGatewayService is a service wrapper that delegates to the message gateway service sdk.
type MessageGatewayService struct {
	region string
	tr     *http.Transport
	signer *v4.Signer
}

// NewService creates a new service instance.
func NewService(log log.T, mgsConfig appconfig.MgsConfig, connectionTimeout time.Duration) Service {

	var region *string
	if mgsConfig.Region != "" {
		region = aws.String(mgsConfig.Region)
	} else {
		fetchedRegion, err := platform.Region()
		if err != nil {
			log.Errorf("Failed to get region with error: %s", err)
		}
		region = &fetchedRegion
	}

	log.Debug("Getting credentials for v4 signatures.")
	var v4Signer *v4.Signer
	creds, _ := getCredentials()
	if creds != nil {
		v4Signer = v4.NewSigner(creds)
	} else {
		log.Debug("Getting credentials for v4 signatures from the metadata service.")

		// load from the metadata service
		metadataCreds := ec2rolecreds.NewCredentials(session.New())
		if metadataCreds != nil {
			v4Signer = v4.NewSigner(metadataCreds)
		} else {
			log.Debug("Failed to get the creds from the metadata service.")
		}
	}

	// capture Transport so we can use it to cancel requests
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   connectionTimeout,
			KeepAlive: 0,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &MessageGatewayService{
		region: aws.StringValue(region),
		tr:     tr,
		signer: v4Signer,
	}
}

// makeRestcall triggers rest api call.
var makeRestcall = func(request []byte, methodType string, url string, region string, signer *v4.Signer) ([]byte, error) {
	httpRequest, err := http.NewRequest(methodType, url, bytes.NewBuffer(request))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %s", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")
	_, err = signer.Sign(httpRequest, bytes.NewReader(request), mgsconfig.ServiceName, region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign the request: %s", err)
	}

	client := &http.Client{
		Timeout: mgsClientTimeout,
	}

	resp, err := client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to make http client call: %s", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes from http response: %s", err)
	}
	if resp.StatusCode == httpStatusCodeCreated {
		return body, nil
	} else {
		return nil, fmt.Errorf("unexpected response from the service %s", body)
	}
}

// getMGSBaseUrl gets the base url of mgs:
// control-channel: https://ssm-messages.{region}.amazonaws.com/v1/control-channel/{channel_id}
// data-channel: https://ssm-messages.{region}.amazonaws.com/v1/data-channel/{session_id}
// channelType can be control-channel or data-channel
func getMGSBaseUrl(log log.T, channelType string, channelId string, region string) (output string, err error) {
	// build url for CreateControlChannel or CreateDataChannel
	hostName := mgsconfig.GetMgsEndpointFromRip(region)
	if hostName == "" {
		return "", fmt.Errorf("failed to get host name with error: %s", err)
	}

	mgsUrl, err := url.Parse(mgsconfig.HttpsPrefix + hostName)
	if err != nil {
		return "", fmt.Errorf("failed to parse the url with error: %s", err)
	}

	mgsUrl.Path = path.Join(mgsUrl.Path, mgsconfig.APIVersion)
	mgsUrl.Path = path.Join(mgsUrl.Path, channelType)
	mgsUrl.Path = path.Join(mgsUrl.Path, channelId)
	return mgsUrl.String(), nil
}

// getCredentials gets the current active credentials.
func getCredentials() (*credentials.Credentials, error) {
	// load managed instance credentials if applicable
	isManaged, err := registration.HasManagedInstancesCredentials()

	if isManaged && err == nil {
		return rolecreds.ManagedInstanceCredentialsInstance(), nil
	}

	return nil, err
}

// GetV4Signer gets the v4 signer.
func (mgsService *MessageGatewayService) GetV4Signer() *v4.Signer {
	return mgsService.signer
}

// GetRegion gets the region.
func (mgsService *MessageGatewayService) GetRegion() string {
	return mgsService.region
}

// CreateControlChannel calls the CreateControlChannel MGS API
func (mgsService *MessageGatewayService) CreateControlChannel(log log.T, createControlChannelInput *CreateControlChannelInput, channelId string) (createControlChannelOutput *CreateControlChannelOutput, err error) {

	url, err := getMGSBaseUrl(log, mgsconfig.ControlChannel, channelId, mgsService.region)
	if err != nil {
		return nil, fmt.Errorf("failed to get the mgs base url with error: %s", err)
	}

	if mgsService.signer == nil {
		return nil, errors.New("MGS service signer is nil")
	}

	jsonValue, err := json.Marshal(createControlChannelInput)
	if err != nil {
		return nil, errors.New("unable to marshal the createControlChannelInput")
	}

	resp, err := makeRestcall(jsonValue, "POST", url, mgsService.region, mgsService.signer)
	if err != nil {
		return nil, fmt.Errorf("createControlChannel request failed: %s", err)
	}

	var output CreateControlChannelOutput
	if resp != nil {
		if err = xml.Unmarshal(resp, &output); err != nil {
			return nil, fmt.Errorf("failed to unmarshal createControlChannel response: %s", err)
		}
		return &output, err
	}
	return nil, err
}

// CreateDataChannel calls the CreateDataChannel MGS API
func (mgsService *MessageGatewayService) CreateDataChannel(log log.T, createDataChannelInput *CreateDataChannelInput, sessionId string) (createDataChannelOutput *CreateDataChannelOutput, err error) {

	url, err := getMGSBaseUrl(log, mgsconfig.DataChannel, sessionId, mgsService.region)
	if err != nil {
		return nil, fmt.Errorf("failed to get the mgs base url with error: %s", err)
	}

	if mgsService.signer == nil {
		return nil, errors.New("MGS service signer is nil")
	}

	jsonValue, err := json.Marshal(createDataChannelInput)
	if err != nil {
		return nil, errors.New("unable to marshal the createDataChannelInput")
	}

	resp, err := makeRestcall(jsonValue, "POST", url, mgsService.region, mgsService.signer)
	if err != nil {
		return nil, fmt.Errorf("createDataChannel request failed: %s", err)
	}

	var output CreateDataChannelOutput
	if resp != nil {
		if err = xml.Unmarshal(resp, &output); err != nil {
			return nil, fmt.Errorf("failed to unmarshal createDataChannel response: %s", err)
		}
		return &output, err
	}

	return nil, err
}
