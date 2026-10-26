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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	cacheExpirationTime = 5 * time.Minute
	metadataTimeout     = 2 * time.Second
)

// gcpMetadataResponse defines the schema for GCP instance metadata
// Note: GCP metadata service returns plain text by default
// JSON format is available with ?alt=json or ?recursive=true query parameters
// Reference: https://cloud.google.com/compute/docs/metadata/querying-metadata
type gcpMetadataResponse struct {
	Instance struct {
		// GCP's metadata service returns the instance ID as a JSON number
		// (e.g. "id": 4520391095309210000) when queried with ?recursive=true.
		// json.Number accepts both the numeric and quoted-string forms without
		// losing precision (GCP IDs are 64-bit and exceed the float64 safe range).
		ID                json.Number `json:"id"`
		Name              string      `json:"name"`
		MachineType       string      `json:"machineType"`
		Zone              string      `json:"zone"`
		Hostname          string      `json:"hostname"`
		Description       string      `json:"description"`
		Tags              []string    `json:"tags"`
		NetworkInterfaces []struct {
			Network       string `json:"network"`
			Subnetwork    string `json:"subnetwork"`
			NetworkIP     string `json:"networkIP"`
			AccessConfigs []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				NatIP string `json:"natIP"`
			} `json:"accessConfigs"`
			IPAliases []string `json:"ipAliases"`
		} `json:"networkInterfaces"`
	} `json:"instance"`
	Project struct {
		ProjectID        string `json:"projectId"`
		NumericProjectID int64  `json:"numericProjectId"`
	} `json:"project"`
}

// cachedMetadata holds the cached metadata and its timestamp
type cachedMetadata struct {
	data      *gcpMetadataResponse
	timestamp time.Time
}

var (
	cache     *cachedMetadata
	cacheLock = &lock
)

// fetchGCPMetadata retrieves metadata from GCP metadata service with caching
func fetchGCPMetadata() (*gcpMetadataResponse, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	// Check if cache is valid
	if cache != nil && time.Since(cache.timestamp) < cacheExpirationTime {
		return cache.data, nil
	}

	// Fetch fresh metadata
	metadata, err := getGCPMetadataResponse()
	if err != nil {
		return nil, err
	}

	// Update cache
	cache = &cachedMetadata{
		data:      metadata,
		timestamp: time.Now(),
	}

	return metadata, nil
}

// getGCPMetadataResponse fetches metadata from GCP metadata service
// Uses ?alt=json query parameter to get JSON format
// Reference: https://cloud.google.com/compute/docs/metadata/querying-metadata#json
func getGCPMetadataResponse() (*gcpMetadataResponse, error) {
	client := &http.Client{
		Timeout: metadataTimeout,
	}

	metadata := &gcpMetadataResponse{}

	// Fetch instance metadata with alt=json to get JSON format
	// GCP returns JSON when using ?alt=json or ?recursive=true
	instanceData, err := fetchGCPMetadataFieldJSON(client, "/instance/?alt=json&recursive=true")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(instanceData, &metadata.Instance); err != nil {
		return nil, fmt.Errorf("unable to parse GCP instance metadata: %v", err)
	}

	// Fetch project metadata with alt=json to get JSON format
	projectData, err := fetchGCPMetadataFieldJSON(client, "/project/?alt=json&recursive=true")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(projectData, &metadata.Project); err != nil {
		return nil, fmt.Errorf("unable to parse GCP project metadata: %v", err)
	}

	return metadata, nil
}

// fetchGCPMetadataFieldJSON fetches a specific field from GCP metadata service in JSON format
func fetchGCPMetadataFieldJSON(client *http.Client, path string) ([]byte, error) {
	req, err := http.NewRequest("GET", appconfig.GCPMetadataEndpoint+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %v", err)
	}

	// GCP metadata service requires this header
	// Reference: https://cloud.google.com/compute/docs/metadata/querying-metadata#request_headers
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get GCP metadata response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GCP metadata: incorrect status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read GCP metadata response body: %v", err)
	}

	return body, nil
}

// fetchInstanceID returns the GCP instance ID
func fetchInstanceID() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Instance.ID.String(), nil
}

// zoneName extracts the bare zone name from a GCP zone value, which the
// metadata service returns as a full path (projects/PROJECT_NUM/zones/ZONE_NAME)
// or, occasionally, as a bare name already.
func zoneName(zone string) string {
	parts := strings.Split(zone, "/")
	return parts[len(parts)-1]
}

// fetchSourceID returns a globally-unique source identifier for the GCP instance
// in the form "projectId:zone:instanceId". GCP numeric instance IDs are only
// unique per zone/region, so the projectId and zone are prefixed to avoid
// collisions on downstream accountId:sourceId indexes.
func fetchSourceID() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s:%s",
		metadata.Project.ProjectID,
		zoneName(metadata.Instance.Zone),
		metadata.Instance.ID.String()), nil
}

// fetchZone returns the GCP zone (full path)
func fetchZone() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Instance.Zone, nil
}

// fetchRegion extracts the region from the zone
func fetchRegion() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}

	zone := metadata.Instance.Zone
	// Zone format: projects/PROJECT_NUM/zones/ZONE_NAME
	// or just: ZONE_NAME (e.g., us-central1-a)
	// Extract region from zone (e.g., us-central1-a -> us-central1)
	parts := strings.Split(zone, "/")
	zoneName := parts[len(parts)-1]

	// Remove the last part after the last hyphen to get region
	lastHyphen := strings.LastIndex(zoneName, "-")
	if lastHyphen == -1 {
		return "", fmt.Errorf("unable to extract region from zone: %s", zone)
	}

	return zoneName[:lastHyphen], nil
}

// fetchInstanceName returns the GCP instance name
func fetchInstanceName() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Instance.Name, nil
}

// fetchCIDRBlock returns the CIDR blocks from GCP network metadata
func fetchCIDRBlock() (map[string][]string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return map[string][]string{}, err
	}

	if len(metadata.Instance.NetworkInterfaces) == 0 {
		return map[string][]string{}, nil
	}

	var ipv4Blocks []string
	var ipv6Blocks []string

	for _, iface := range metadata.Instance.NetworkInterfaces {
		// GCP primarily uses IPv4, collect network IPs
		if iface.NetworkIP != "" {
			ipv4Blocks = append(ipv4Blocks, iface.NetworkIP)
		}
		// Collect IP aliases if any
		for _, alias := range iface.IPAliases {
			if strings.Contains(alias, ":") {
				ipv6Blocks = append(ipv6Blocks, alias)
			} else {
				ipv4Blocks = append(ipv4Blocks, alias)
			}
		}
	}

	return map[string][]string{"ipv4": ipv4Blocks, "ipv6": ipv6Blocks}, nil
}

// fetchComputerName returns a composite computer name for the GCP instance.
// Format: projectId.instanceName
func fetchComputerName() (string, error) {
	metadata, err := fetchGCPMetadata()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", metadata.Project.ProjectID, metadata.Instance.Name), nil
}
