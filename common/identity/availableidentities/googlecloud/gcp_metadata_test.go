// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package googlecloud

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetCache clears the metadata cache before each test.
func resetCache() {
	cacheLock.Lock()
	cache = nil
	cacheLock.Unlock()
}

// newMetadataServer starts an httptest server that serves instanceJSON from /instance/*
// and projectJSON from /project/*, and validates the required Metadata-Flavor header.
func newMetadataServer(t *testing.T, instanceJSON, projectJSON string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Google", r.Header.Get("Metadata-Flavor"),
			"Metadata-Flavor header must be 'Google'")

		switch {
		case r.URL.Path == "/instance/id":
			// plain-text probe used by checkGCPIMDSAvailable
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("1234567890"))
		case len(r.URL.Path) >= len("/instance") && r.URL.Path[:len("/instance")] == "/instance":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(instanceJSON))
		case len(r.URL.Path) >= len("/project") && r.URL.Path[:len("/project")] == "/project":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(projectJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// withMockEndpoint points GCPMetadataEndpoint at the test server for the
// duration of the test and restores it afterwards.
func withMockEndpoint(t *testing.T, server *httptest.Server) {
	t.Helper()
	orig := appconfig.GCPMetadataEndpoint
	appconfig.GCPMetadataEndpoint = server.URL
	t.Cleanup(func() { appconfig.GCPMetadataEndpoint = orig })
}

// sampleInstanceJSON is a realistic GCP instance metadata payload.
const sampleInstanceJSON = `{
	"id": "1234567890",
	"name": "my-instance",
	"machineType": "projects/123456/machineTypes/n1-standard-2",
	"zone": "projects/123456/zones/us-central1-a",
	"hostname": "my-instance.c.my-project.internal",
	"description": "",
	"tags": [],
	"networkInterfaces": [
		{
			"network": "projects/123456/global/networks/default",
			"subnetwork": "projects/123456/regions/us-central1/subnetworks/default",
			"networkIP": "10.128.0.2",
			"accessConfigs": [
				{"type": "ONE_TO_ONE_NAT", "name": "External NAT", "natIP": "35.1.2.3"}
			],
			"ipAliases": ["192.168.1.0/24"]
		}
	]
}`

// sampleProjectJSON is a realistic GCP project metadata payload.
const sampleProjectJSON = `{
	"projectId": "my-project",
	"numericProjectId": 123456
}`

// --- fetchGCPMetadata / getGCPMetadataResponse ---

func TestFetchGCPMetadata_ReturnsCorrectData(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	metadata, err := fetchGCPMetadata()

	require.NoError(t, err)
	assert.Equal(t, "1234567890", metadata.Instance.ID.String())
	assert.Equal(t, "my-instance", metadata.Instance.Name)
	assert.Equal(t, "projects/123456/machineTypes/n1-standard-2", metadata.Instance.MachineType)
	assert.Equal(t, "projects/123456/zones/us-central1-a", metadata.Instance.Zone)
	assert.Equal(t, "my-project", metadata.Project.ProjectID)
	assert.Equal(t, int64(123456), metadata.Project.NumericProjectID)
}

// TestFetchGCPMetadata_ParsesNumericInstanceID is a regression test for the
// case where GCP's metadata service (?recursive=true) returns the instance id
// as a JSON number rather than a quoted string. Previously this failed with
// "json: cannot unmarshal number into Go struct field .id of type string".
func TestFetchGCPMetadata_ParsesNumericInstanceID(t *testing.T) {
	resetCache()
	numericInstanceJSON := `{
		"id": 4520391095309210000,
		"name": "my-instance",
		"zone": "projects/123456/zones/us-central1-a"
	}`
	server := newMetadataServer(t, numericInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	metadata, err := fetchGCPMetadata()

	require.NoError(t, err)
	assert.Equal(t, "4520391095309210000", metadata.Instance.ID.String(),
		"numeric id must be parsed without precision loss")
}

func TestFetchGCPMetadata_ReturnsError_WhenServerUnreachable(t *testing.T) {
	resetCache()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := server.URL
	server.Close()

	orig := appconfig.GCPMetadataEndpoint
	appconfig.GCPMetadataEndpoint = endpoint
	defer func() { appconfig.GCPMetadataEndpoint = orig }()

	_, err := fetchGCPMetadata()
	assert.Error(t, err)
}

func TestFetchGCPMetadata_ReturnsError_WhenInstanceResponseInvalidJSON(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, `{invalid json}`, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	_, err := fetchGCPMetadata()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse GCP instance metadata")
}

func TestFetchGCPMetadata_ReturnsError_WhenProjectResponseInvalidJSON(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, `{bad}`)
	defer server.Close()
	withMockEndpoint(t, server)

	_, err := fetchGCPMetadata()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse GCP project metadata")
}

// --- cache behaviour ---

func TestFetchGCPMetadata_UsesCachedData(t *testing.T) {
	resetCache()

	// Prime the cache directly — no HTTP server needed.
	primed := &gcpMetadataResponse{}
	primed.Instance.ID = "cached-id"
	primed.Project.ProjectID = "cached-project"
	cacheLock.Lock()
	cache = &cachedMetadata{data: primed, timestamp: time.Now()}
	cacheLock.Unlock()

	// Serve different data; it must NOT be fetched.
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	metadata, err := fetchGCPMetadata()
	require.NoError(t, err)
	assert.Equal(t, "cached-id", metadata.Instance.ID.String(), "should return cached data, not fresh HTTP response")
	assert.Equal(t, "cached-project", metadata.Project.ProjectID)
}

func TestFetchGCPMetadata_RefreshesExpiredCache(t *testing.T) {
	resetCache()

	stale := &gcpMetadataResponse{}
	stale.Instance.ID = "stale-id"
	cacheLock.Lock()
	cache = &cachedMetadata{data: stale, timestamp: time.Now().Add(-6 * time.Minute)}
	cacheLock.Unlock()

	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	metadata, err := fetchGCPMetadata()
	require.NoError(t, err)
	assert.Equal(t, "1234567890", metadata.Instance.ID.String(), "should fetch fresh data after cache expiry")
	assert.Equal(t, "my-project", metadata.Project.ProjectID)
}

// --- fetch helper functions ---

func TestFetchInstanceID_ReturnsCorrectValue(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	id, err := fetchInstanceID()
	require.NoError(t, err)
	assert.Equal(t, "1234567890", id)
}

func TestFetchSourceID_ReturnsProjectZoneInstance(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	sourceID, err := fetchSourceID()
	require.NoError(t, err)
	// projectId:zone:instanceId, with the bare zone name (not the full path).
	assert.Equal(t, "my-project:us-central1-a:1234567890", sourceID)
}

func TestFetchSourceID_HandlesBareZoneName(t *testing.T) {
	resetCache()
	instanceJSON := `{
		"id": 4520391095309210000,
		"name": "my-instance",
		"zone": "us-central1-a"
	}`
	server := newMetadataServer(t, instanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	sourceID, err := fetchSourceID()
	require.NoError(t, err)
	assert.Equal(t, "my-project:us-central1-a:4520391095309210000", sourceID)
}

func TestFetchZone_ReturnsFullZonePath(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	zone, err := fetchZone()
	require.NoError(t, err)
	assert.Equal(t, "projects/123456/zones/us-central1-a", zone)
}

func TestFetchInstanceName_ReturnsCorrectValue(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	name, err := fetchInstanceName()
	require.NoError(t, err)
	assert.Equal(t, "my-instance", name)
}

func TestFetchComputerName_ReturnsProjectDotInstance(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	name, err := fetchComputerName()
	require.NoError(t, err)
	assert.Equal(t, "my-project.my-instance", name)
}

// --- fetchRegion zone format table test ---

func TestFetchRegion_ZoneFormats(t *testing.T) {
	cases := []struct {
		name     string
		zone     string
		expected string
		wantErr  bool
	}{
		{"full GCE path", "projects/123/zones/us-central1-a", "us-central1", false},
		{"bare zone name", "us-central1-a", "us-central1", false},
		{"europe zone", "europe-west1-b", "europe-west1", false},
		{"asia zone", "asia-east1-c", "asia-east1", false},
		{"no hyphen", "invalid", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetCache()
			cacheLock.Lock()
			mock := &gcpMetadataResponse{}
			mock.Instance.Zone = tc.zone
			cache = &cachedMetadata{data: mock, timestamp: time.Now()}
			cacheLock.Unlock()

			region, err := fetchRegion()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, region)
			}
		})
	}
}

// --- fetchCIDRBlock ---

func TestFetchCIDRBlock_ReturnsIPv4Blocks(t *testing.T) {
	resetCache()
	server := newMetadataServer(t, sampleInstanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	blocks, err := fetchCIDRBlock()
	require.NoError(t, err)
	assert.Contains(t, blocks["ipv4"], "10.128.0.2")
	assert.Contains(t, blocks["ipv4"], "192.168.1.0/24")
	assert.Empty(t, blocks["ipv6"])
}

func TestFetchCIDRBlock_ReturnsIPv6Blocks(t *testing.T) {
	resetCache()
	instanceJSON := `{
		"id": "999",
		"name": "ipv6-instance",
		"machineType": "projects/1/machineTypes/f1-micro",
		"zone": "us-central1-a",
		"networkInterfaces": [
			{
				"networkIP": "10.0.0.1",
				"ipAliases": ["2001:db8::1/64", "172.16.0.0/12"]
			}
		]
	}`
	server := newMetadataServer(t, instanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	blocks, err := fetchCIDRBlock()
	require.NoError(t, err)
	assert.Contains(t, blocks["ipv4"], "10.0.0.1")
	assert.Contains(t, blocks["ipv4"], "172.16.0.0/12")
	assert.Contains(t, blocks["ipv6"], "2001:db8::1/64")
}

func TestFetchCIDRBlock_ReturnsEmpty_WhenNoInterfaces(t *testing.T) {
	resetCache()
	instanceJSON := `{
		"id": "111",
		"name": "no-net",
		"machineType": "projects/1/machineTypes/f1-micro",
		"zone": "us-central1-a",
		"networkInterfaces": []
	}`
	server := newMetadataServer(t, instanceJSON, sampleProjectJSON)
	defer server.Close()
	withMockEndpoint(t, server)

	blocks, err := fetchCIDRBlock()
	require.NoError(t, err)
	assert.Empty(t, blocks["ipv4"])
	assert.Empty(t, blocks["ipv6"])
}

// --- HTTP error handling ---

func TestFetchGCPMetadataFieldJSON_ReturnsError_WhenStatusNotOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	orig := appconfig.GCPMetadataEndpoint
	appconfig.GCPMetadataEndpoint = server.URL
	defer func() { appconfig.GCPMetadataEndpoint = orig }()

	client := &http.Client{Timeout: metadataTimeout}
	_, err := fetchGCPMetadataFieldJSON(client, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incorrect status code 403")
}
