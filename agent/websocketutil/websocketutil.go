// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package websocketutil contains methods for interacting with websocket connections.
package websocketutil

import (
	"errors"
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/gorilla/websocket"
)

// IWebsocketUtil is the interface for the websocketutil.
type IWebsocketUtil interface {
	OpenConnection(url string, requestHeader http.Header) (*websocket.Conn, error)
	CloseConnection(ws *websocket.Conn) error
}

// WebsocketUtil struct provides functionality around creating and maintaining websockets.
type WebsocketUtil struct {
	dialer *websocket.Dialer
	log    log.T
}

// NewWebsocketUtil is the factory function for websocketutil.
func NewWebsocketUtil(logger log.T, dialerInput *websocket.Dialer) *WebsocketUtil {

	var websocketUtil *WebsocketUtil

	if dialerInput == nil {
		websocketUtil = &WebsocketUtil{
			dialer: websocket.DefaultDialer,
			log:    logger,
		}
	} else {
		websocketUtil = &WebsocketUtil{
			dialer: dialerInput,
			log:    logger,
		}
	}

	return websocketUtil
}

// OpenConnection opens a websocket connection provided an input url and request header.
func (u *WebsocketUtil) OpenConnection(url string, requestHeader http.Header) (*websocket.Conn, error) {

	u.log.Infof("Opening websocket connection to: %s", url)

	conn, _, err := u.dialer.Dial(url, requestHeader)
	if err != nil {
		u.log.Warnf("Failed to dial websocket: %s", err)
		return nil, err
	}

	u.log.Infof("Successfully opened websocket connection to: %s", url)

	return conn, err
}

// CloseConnection closes a websocket connection given the Conn object as input.
func (u *WebsocketUtil) CloseConnection(ws *websocket.Conn) error {
	if ws == nil {
		return errors.New("websocket conn object is nil")
	}

	u.log.Debugf("Closing websocket connection to: %s", ws.RemoteAddr())

	err := ws.Close()
	if err != nil {
		u.log.Warnf("Failed to close websocket: %s", err)
		return err
	}

	u.log.Infof("Successfully closed websocket connection to: %s", ws.RemoteAddr())

	return nil
}
