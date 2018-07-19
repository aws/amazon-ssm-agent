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
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsconfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/aws/amazon-ssm-agent/agent/websocketutil"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gorilla/websocket"
)

// IWebSocketChannel is the interface for ControlChannel and DataChannel.
type IWebSocketChannel interface {
	Open(log log.T) error
	Close(log log.T) error
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

// Open upgrades the http connection to a websocket connection.
func (webSocketChannel *WebSocketChannel) Open(log log.T) error {

	// initialize the write mutex
	webSocketChannel.writeLock = &sync.Mutex{}

	header, err := webSocketChannel.getV4SignatureHeader(log, webSocketChannel.Url)
	if err != nil {
		log.Errorf("Failed to get the v4 signature, %v", err)
	}

	headerString := header.Get("Authorization")
	log.Debug(headerString)

	ws, err := websocketutil.NewWebsocketUtil(log, nil).OpenConnection(webSocketChannel.Url, header)
	if err != nil {
		return err
	}

	webSocketChannel.Connection = ws
	webSocketChannel.IsOpen = true
	webSocketChannel.StartPings(log, time.Minute)

	// spin up a different routine to listen to the incoming traffic
	go func() {

		defer func() {
			if msg := recover(); msg != nil {
				log.Errorf("WebsocketChannel listener run panic: %v", msg)
				log.Errorf("%s: %s", msg, debug.Stack())
			}
		}()

		retryCount := 0
		for {
			log.Tracef("Receiving message from websocket channel %s", webSocketChannel.ChannelToken)

			if webSocketChannel.IsOpen == false {
				log.Info("Ending the channel listening routine since the channel is closed")
				break
			}

			messageType, rawMessage, err := webSocketChannel.Connection.ReadMessage()
			if err != nil {
				retryCount++
				if retryCount >= mgsconfig.RetryAttempt {
					log.Errorf("Reach the retry limit %v for receive messages. Error: %v", mgsconfig.RetryAttempt, err.Error())
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
				log.Errorf("Invalid message type %s. We only accept UTF-8 or binary encoded text", messageType)

			} else {
				retryCount = 0
				log.Tracef("Message %s received.", string(rawMessage))

				webSocketChannel.OnMessage(rawMessage)
			}
		}
	}()

	return nil
}

// StartPings starts the pinging process to keep the websocket channel alive.
func (webSocketChannel *WebSocketChannel) StartPings(log log.T, pingInterval time.Duration) {

	go func() {
		for {
			if webSocketChannel.IsOpen == false {
				return
			}

			log.Debug("WebsocketChannel: Send ping. Message.")
			webSocketChannel.writeLock.Lock()
			err := webSocketChannel.Connection.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			webSocketChannel.writeLock.Unlock()
			if err != nil {
				log.Errorf("Error while sending websocket ping: %v", err)
				return
			}
			time.Sleep(pingInterval)
		}
	}()
}

// Close closes the corresponding connection.
func (webSocketChannel *WebSocketChannel) Close(log log.T) error {

	log.Info("Closing websocket channel connection to: " + webSocketChannel.Url)
	if webSocketChannel.IsOpen == true {
		// Send signal to stop receiving message
		webSocketChannel.IsOpen = false
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
	err := webSocketChannel.Connection.WriteMessage(inputType, input)
	webSocketChannel.writeLock.Unlock()
	return err
}
