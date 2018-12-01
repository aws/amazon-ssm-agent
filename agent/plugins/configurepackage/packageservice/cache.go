// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package packageservice

import (
	"fmt"
)

// ManifestCache caches manifests locally
type ManifestCache interface {
	ReadManifest(packageArn string, packageVersion string) ([]byte, error)
	WriteManifest(packageArn string, packageVersion string, content []byte) error
	ReadManifestHash(packageArn string, documentVersion string) ([]byte, error)
	WriteManifestHash(packageArn string, documentVersion string, content []byte) error
}

// ManifestCacheMem stores cache in memory
type ManifestCacheMem struct {
	cache map[string][]byte
}

func ManifestCacheMemNew() *ManifestCacheMem {
	return &ManifestCacheMem{cache: map[string][]byte{}}
}

func (c ManifestCacheMem) CacheKey(packageArn string, packageVersion string) string {
	return fmt.Sprintf("%s_%s", packageArn, packageVersion)
}

func (c ManifestCacheMem) ReadManifest(packageArn string, packageVersion string) ([]byte, error) {
	return c.cache[c.CacheKey(packageArn, packageVersion)], nil
}

func (c ManifestCacheMem) WriteManifest(packageArn string, packageVersion string, content []byte) error {
	c.cache[c.CacheKey(packageArn, packageVersion)] = content
	return nil
}

func (c ManifestCacheMem) ReadManifestHash(packageArn string, documentVersion string) ([]byte, error) {
	return c.cache[c.CacheKey(packageArn, documentVersion)], nil
}

func (c ManifestCacheMem) WriteManifestHash(packageArn string, documentVersion string, content []byte) error {
	c.cache[c.CacheKey(packageArn, documentVersion)] = content
	return nil
}
