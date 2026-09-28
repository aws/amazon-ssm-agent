// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/

package identity

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
)

// --- EC2 IMDS tests ---

func TestCheckEC2IMDSAvailable_ReturnsTrue_WhenIMDSResponds(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "")
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-aws-ec2-metadata-token-ttl-seconds", "21600")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock-token"))
	})
	mux.HandleFunc("/latest/meta-data/instance-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("i-1234567890abcdef0"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	original := newEC2IMDSClient
	newEC2IMDSClient = func() (*imds.Client, error) {
		client := imds.New(imds.Options{
			Endpoint: server.URL,
		})
		return client, nil
	}
	defer func() { newEC2IMDSClient = original }()

	assert.True(t, checkEC2IMDSAvailable())
}

func TestCheckEC2IMDSAvailable_ReturnsFalse_WhenIMDSReturns404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-aws-ec2-metadata-token-ttl-seconds", "21600")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("mock-token"))
	})
	mux.HandleFunc("/latest/meta-data/instance-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	original := newEC2IMDSClient
	newEC2IMDSClient = func() (*imds.Client, error) {
		client := imds.New(imds.Options{
			Endpoint: server.URL,
		})
		return client, nil
	}
	defer func() { newEC2IMDSClient = original }()

	assert.False(t, checkEC2IMDSAvailable())
}

func TestCheckEC2IMDSAvailable_ReturnsFalse_WhenUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := server.URL
	server.Close()

	original := newEC2IMDSClient
	newEC2IMDSClient = func() (*imds.Client, error) {
		client := imds.New(imds.Options{
			Endpoint: endpoint,
		})
		return client, nil
	}
	defer func() { newEC2IMDSClient = original }()

	assert.False(t, checkEC2IMDSAvailable())
}

func TestCheckEC2IMDSAvailable_ReturnsFalse_WhenClientCreationFails(t *testing.T) {
	original := newEC2IMDSClient
	newEC2IMDSClient = func() (*imds.Client, error) {
		return nil, fmt.Errorf("client creation failed")
	}
	defer func() { newEC2IMDSClient = original }()

	assert.False(t, checkEC2IMDSAvailable())
}

// --- Azure IMDS tests ---

func TestCheckAzureIMDSAvailable_ReturnsTrue_WhenIMDSResponds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.Header.Get("Metadata"))
		assert.Equal(t, appconfig.AzureIMDSAPIVersion, r.URL.Query().Get("api-version"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"compute":{"vmId":"test-vm-id"}}`))
	}))
	defer server.Close()

	origEndpoint := appconfig.AzureIMDSEndpoint
	appconfig.AzureIMDSEndpoint = server.URL
	defer func() { appconfig.AzureIMDSEndpoint = origEndpoint }()

	assert.True(t, checkAzureIMDSAvailable())
}

func TestCheckAzureIMDSAvailable_ReturnsFalse_WhenIMDSReturns404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origEndpoint := appconfig.AzureIMDSEndpoint
	appconfig.AzureIMDSEndpoint = server.URL
	defer func() { appconfig.AzureIMDSEndpoint = origEndpoint }()

	assert.False(t, checkAzureIMDSAvailable())
}

func TestCheckAzureIMDSAvailable_ReturnsFalse_WhenUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := server.URL
	server.Close()

	origEndpoint := appconfig.AzureIMDSEndpoint
	appconfig.AzureIMDSEndpoint = endpoint
	defer func() { appconfig.AzureIMDSEndpoint = origEndpoint }()

	assert.False(t, checkAzureIMDSAvailable())
}
