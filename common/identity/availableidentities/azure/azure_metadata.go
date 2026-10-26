// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

var lock sync.RWMutex

const (
	cacheExpirationTime = 5 * time.Minute
	metadataTimeout     = 2 * time.Second
)

// azureMetadataResponse defines the schema for Azure instance metadata
type azureMetadataResponse struct {
	Compute struct {
		AzEnvironment        string `json:"azEnvironment"`
		Location             string `json:"location"`
		Name                 string `json:"name"`
		Offer                string `json:"offer"`
		OsType               string `json:"osType"`
		PlacementGroupID     string `json:"placementGroupId"`
		PlatformFaultDomain  string `json:"platformFaultDomain"`
		PlatformUpdateDomain string `json:"platformUpdateDomain"`
		Publisher            string `json:"publisher"`
		ResourceGroupName    string `json:"resourceGroupName"`
		ResourceID           string `json:"resourceId"`
		Sku                  string `json:"sku"`
		StorageProfile       struct {
			ImageReference struct {
				ID        string `json:"id"`
				Offer     string `json:"offer"`
				Publisher string `json:"publisher"`
				Sku       string `json:"sku"`
				Version   string `json:"version"`
			} `json:"imageReference"`
		} `json:"storageProfile"`
		SubscriptionID string `json:"subscriptionId"`
		Tags           string `json:"tags"`
		Version        string `json:"version"`
		VMID           string `json:"vmId"`
		VMScaleSetName string `json:"vmScaleSetName"`
		VMSize         string `json:"vmSize"`
		Zone           string `json:"zone"`
	} `json:"compute"`
	Network struct {
		Interface []struct {
			IPv4 struct {
				IPAddress []struct {
					PrivateIPAddress string `json:"privateIpAddress"`
					PublicIPAddress  string `json:"publicIpAddress"`
				} `json:"ipAddress"`
				Subnet []struct {
					Address string `json:"address"`
					Prefix  string `json:"prefix"`
				} `json:"subnet"`
			} `json:"ipv4"`
			IPv6 struct {
				IPAddress []struct {
					PrivateIPAddress string `json:"privateIpAddress"`
				} `json:"ipAddress"`
			} `json:"ipv6"`
			MacAddress string `json:"macAddress"`
		} `json:"interface"`
	} `json:"network"`
}

// cachedMetadata holds the cached metadata and its timestamp
type cachedMetadata struct {
	data      *azureMetadataResponse
	timestamp time.Time
}

var (
	cache     *cachedMetadata
	cacheLock = &lock
)

// fetchAzureMetadata retrieves metadata from Azure IMDS with caching
func fetchAzureMetadata() (*azureMetadataResponse, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	// Check if cache is valid
	if cache != nil && time.Since(cache.timestamp) < cacheExpirationTime {
		return cache.data, nil
	}

	// Fetch fresh metadata
	metadata, err := getAzureMetadataResponse()
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

// getAzureMetadataResponse fetches metadata from Azure IMDS
func getAzureMetadataResponse() (*azureMetadataResponse, error) {
	client := &http.Client{
		Timeout: metadataTimeout,
	}

	req, err := http.NewRequest("GET", appconfig.AzureIMDSEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %v", err)
	}

	// Azure IMDS requires these headers
	req.Header.Add("Metadata", "true")
	q := req.URL.Query()
	q.Add("format", "json")
	q.Add("api-version", appconfig.AzureIMDSAPIVersion)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to get Azure metadata response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("imds Azure metadata: incorrect status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read Azure metadata response body: %v", err)
	}

	var metadata azureMetadataResponse
	err = json.Unmarshal(body, &metadata)
	if err != nil {
		return nil, fmt.Errorf("unable to parse Azure metadata response: %v", err)
	}

	return &metadata, nil
}

// fetchVMID returns the Azure VM ID
func fetchVMID() (string, error) {
	metadata, err := fetchAzureMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Compute.VMID, nil
}

// fetchRegion returns the Azure region (location)
func fetchRegion() (string, error) {
	metadata, err := fetchAzureMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Compute.Location, nil
}

// fetchAvailabilityZone returns the Azure availability zone
func fetchAvailabilityZone() (string, error) {
	metadata, err := fetchAzureMetadata()
	if err != nil {
		return "", err
	}
	return metadata.Compute.Zone, nil
}

func fetchComputerName() (string, error) {
	metadata, err := fetchAzureMetadata()
	if err != nil {
		return "", err
	}
	computerName := fmt.Sprintf("%s.%s.%s", metadata.Compute.SubscriptionID, metadata.Compute.ResourceGroupName, metadata.Compute.Name)
	return computerName, nil
}

// fetchCIDRBlock returns the CIDR blocks from Azure network metadata
func fetchCIDRBlock() (map[string][]string, error) {
	metadata, err := fetchAzureMetadata()
	if err != nil {
		return map[string][]string{}, err
	}

	if len(metadata.Network.Interface) == 0 {
		return map[string][]string{}, nil
	}

	var ipv4Blocks []string
	var ipv6Blocks []string

	for _, iface := range metadata.Network.Interface {
		for _, subnet := range iface.IPv4.Subnet {
			if subnet.Prefix != "" {
				ipv4Blocks = append(ipv4Blocks, subnet.Address+"/"+subnet.Prefix)
			}
		}
		// Azure IPv6 subnet info is typically not available in the same format
		// but we can collect IPv6 addresses if needed
		if len(iface.IPv6.IPAddress) > 0 {
			for _, ipv6 := range iface.IPv6.IPAddress {
				if ipv6.PrivateIPAddress != "" {
					ipv6Blocks = append(ipv6Blocks, ipv6.PrivateIPAddress)
				}
			}
		}
	}

	return map[string][]string{"ipv4": ipv4Blocks, "ipv6": ipv6Blocks}, nil
}
