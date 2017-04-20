package birdwatcher

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// ManifestCache caches manifests locally
type ManifestCache interface {
	ReadManifest(packageName string, packageVersion string) ([]byte, error)
	WriteManifest(packageName string, packageVersion string, content []byte) error
}

// ManifestCacheDisk stores cache on disk
type ManifestCacheDisk struct {
	CachePath string
}

func (c ManifestCacheDisk) FilePath(packageName string, packageVersion string) string {
	return filepath.Join(c.CachePath, fmt.Sprintf("%s_%s.json", packageName, packageVersion))
}

func (c ManifestCacheDisk) ReadManifest(packageName string, packageVersion string) ([]byte, error) {
	return ioutil.ReadFile(c.FilePath(packageName, packageVersion))
}

func (c ManifestCacheDisk) WriteManifest(packageName string, packageVersion string, content []byte) error {
	return ioutil.WriteFile(c.FilePath(packageName, packageVersion), content, 0600)
}

// ManifestCacheMem stores cache in memory
type ManifestCacheMem struct {
	cache map[string][]byte
}

func (c ManifestCacheMem) CacheKey(packageName string, packageVersion string) string {
	return fmt.Sprintf("%s_%s", packageName, packageVersion)
}

func (c ManifestCacheMem) ReadManifest(packageName string, packageVersion string) ([]byte, error) {
	return c.cache[c.CacheKey(packageName, packageVersion)], nil
}

func (c ManifestCacheMem) WriteManifest(packageName string, packageVersion string, content []byte) error {
	c.cache[c.CacheKey(packageName, packageVersion)] = content
	return nil
}
