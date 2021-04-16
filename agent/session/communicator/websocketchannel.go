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

// communicator package implement base communicator for network connections.
package communicator

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator/websocketutil"
	mgsconfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gorilla/websocket"
)

// IWebSocketChannel is the interface for ControlChannel and DataChannel.
type IWebSocketChannel interface {
	Initialize(context context.T,
		channelId string,
		channelType string,
		channelRole string,
		channelToken string,
		region string,
		signer *v4.Signer,
		onMessageHandler func([]byte),
		onErrorHandler func(error)) error
	Open(log log.T) error
	Close(log log.T) error
	GetChannelToken() string
	SetChannelToken(token string)
	StartPings(log log.T, pingInterval time.Duration)
	SendMessage(log log.T, input []byte, inputType int) error
	SetUrl(url string)
	SetSubProtocol(subProtocol string)
}

// WebSocketChannel parent class for ControlChannel and DataChannel.
type WebSocketChannel struct {
	OnMessage    func([]byte)
	OnError      func(error)
	Context      context.T
	ChannelToken string
	Connection   *websocket.Conn
	Url          string
	SubProtocol  string
	Signer       *v4.Signer
	Region       string
	IsOpen       bool
	writeLock    *sync.Mutex
	stopPinging  chan bool
}

// Initialize a WebSocketChannel object.
func (webSocketChannel *WebSocketChannel) Initialize(context context.T,
	channelId string,
	channelType string,
	channelRole string,
	channelToken string,
	region string,
	signer *v4.Signer,
	onMessageHandler func([]byte),
	onErrorHandler func(error)) error {

	hostName := mgsconfig.GetMgsEndpointFromRip(context, region)
	if hostName == "" {
		return fmt.Errorf("no MGS endpoint found")
	}

	channelUrl, err := url.Parse(mgsconfig.WebSocketPrefix + hostName)
	if err != nil {
		return err
	}

	channelUrl.Path = path.Join(channelUrl.Path, mgsconfig.APIVersion)
	channelUrl.Path = path.Join(channelUrl.Path, channelType)
	channelUrl.Path = path.Join(channelUrl.Path, channelId)

	query := channelUrl.Query()
	if channelType == mgsconfig.ControlChannel {
		query.Set(mgsconfig.StreamQueryParameter, "input")
		query.Add(mgsconfig.RoleQueryParameter, channelRole)
	} else if channelType == mgsconfig.DataChannel {
		query.Set(mgsconfig.RoleQueryParameter, channelRole)
	}

	channelUrl.RawQuery = query.Encode()

	webSocketChannel.Url = channelUrl.String()
	webSocketChannel.Context = context
	webSocketChannel.Region = region
	webSocketChannel.Signer = signer
	webSocketChannel.ChannelToken = channelToken
	webSocketChannel.OnError = onErrorHandler
	webSocketChannel.OnMessage = onMessageHandler

	return nil
}

// SetUrl sets the url for the WebSocketChannel.
func (webSocketChannel *WebSocketChannel) SetUrl(url string) {
	webSocketChannel.Url = url
}

// SetSubProtocol sets the subprotocol for the WebSocketChannel.
func (webSocketChannel *WebSocketChannel) SetSubProtocol(subProtocol string) {
	webSocketChannel.SubProtocol = subProtocol
}

// getV4SignatureHeader gets the signed header.
func (webSocketChannel *WebSocketChannel) getV4SignatureHeader(log log.T, Url string) (http.Header, error) {

	request, err := http.NewRequest("GET", Url, nil)

	if webSocketChannel.Signer != nil {
		_, err = webSocketChannel.Signer.Sign(request, nil, mgsconfig.ServiceName, webSocketChannel.Region, time.Now())
	}
	return request.Header, err
}

// GetChannelToken returns channelToken field.
func (webSocketChannel *WebSocketChannel) GetChannelToken() string {
	return webSocketChannel.ChannelToken
}

// SetChannelToken updates the token field.
func (webSocketChannel *WebSocketChannel) SetChannelToken(token string) {
	webSocketChannel.ChannelToken = token
}

// Open upgrades the http connection to a websocket connection.
func (webSocketChannel *WebSocketChannel) Open(log log.T) error {

	// initialize the write mutex
	webSocketChannel.writeLock = &sync.Mutex{}

	header, err := webSocketChannel.getV4SignatureHeader(log, webSocketChannel.Url)
	if err != nil {
		log.Errorf("Failed to get the v4 signature, %v", err)
	}

	ws, err := websocketutil.NewWebsocketUtil(log, nil).OpenConnection(webSocketChannel.Url, header)
	if err != nil {
		return err
	}

	webSocketChannel.Connection = ws
	webSocketChannel.IsOpen = true
	webSocketChannel.stopPinging = make(chan bool, 1)
	webSocketChannel.StartPings(log, mgsconfig.WebSocketPingInterval)

	// spin up a different routine to listen to the incoming traffic
	go func() {
		defer log.Info("Ending websocket listener")
		log.Info("Starting websocket listener")

		defer func() {
			if msg := recover(); msg != nil {
				log.Errorf("WebsocketChannel listener run panic: %v", msg)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()

		retryCount := 0
		for {

			if webSocketChannel.IsOpen == false {
				log.Info("Ending the channel listening routine since the channel is closed")
				break
			}

			messageType, rawMessage, err := webSocketChannel.Connection.ReadMessage()
			if err != nil {
				retryCount++
				if retryCount >= mgsconfig.RetryAttempt {
					log.Warnf("Reach the retry limit %v for receive messages. Error: %v", mgsconfig.RetryAttempt, err.Error())
					webSocketChannel.OnError(err)
					break
				}
				log.Debugf(
					"An error happened when receiving the message. Retried times: %d, MessageType: %v, Error: %s",
					retryCount,
					messageType,
					err.Error())

			} else if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
				// We only accept text messages which are interpreted as UTF-8 or binary encoded text.
				log.Errorf("Invalid message type %d. We only accept UTF-8 or binary encoded text", messageType)

			} else {
				retryCount = 0

				webSocketChannel.OnMessage(rawMessage)
			}
		}
	}()

	return nil
}

// StartPings starts the pinging process to keep the websocket channel alive.
func (webSocketChannel *WebSocketChannel) StartPings(log log.T, pingInterval time.Duration) {

	go func(done chan bool) {
		log.Info("Starting websocket pinger")
		defer log.Info("Ending websocket pinger")

		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Websocket channel start pings panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return

			case <-ticker.C:
				log.Debug("WebsocketChannel: Send ping. Message.")
				err := webSocketChannel.SendMessage(log, []byte("keepalive"), websocket.PingMessage)
				if err != nil {
					log.Warnf("Error while sending websocket ping: %v", err)
					return
				}
			}
		}
	}(webSocketChannel.stopPinging) // explicitly passed in case it changes on Close/Reopen
}

// Close closes the corresponding connection.
func (webSocketChannel *WebSocketChannel) Close(log log.T) error {

	log.Info("Closing websocket channel connection to: " + webSocketChannel.Url)

	// Send signal to stop receiving message
	if webSocketChannel.IsOpen == true {
		webSocketChannel.IsOpen = false

		webSocketChannel.stopPinging <- true
		close(webSocketChannel.stopPinging)

		return websocketutil.NewWebsocketUtil(log, nil).CloseConnection(webSocketChannel.Connection)
	}

	log.Debugf("Websocket channel connection to: " + webSocketChannel.Url + " is already Closed!")
	return nil
}

// SendMessage sends a byte message through the websocket connection.
// Examples of message type are websocket.TextMessage or websocket.Binary
func (webSocketChannel *WebSocketChannel) SendMessage(log log.T, input []byte, inputType int) error {
	if webSocketChannel.IsOpen == false {
		return errors.New("Can't send message: Connection is closed.")
	}

	if len(input) < 1 {
		return errors.New("Can't send message: Empty input.")
	}

	webSocketChannel.writeLock.Lock()
	defer webSocketChannel.writeLock.Unlock()
	err := webSocketChannel.Connection.WriteMessage(inputType, input)
	return err
}
