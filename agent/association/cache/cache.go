package cache

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
)

type Cache struct {
	associaions map[string]*model.InstanceAssociation
	mutex       sync.RWMutex
}

var cache *Cache
var once sync.Once

func init() {
	once.Do(func() {
		assoDetailsMap := make(map[string]*model.InstanceAssociation)
		cache = &Cache{
			associaions: assoDetailsMap,
		}
	})
}

// GetCache returns a singleton of CloudWatchConfig instance
func GetCache() *Cache {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	return cache
}

// set sets the value to the target key in map
func (c *Cache) set(associationID string, associationRawData *model.InstanceAssociation) {
	c.associaions[associationID] = associationRawData
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *Cache) Add(associationID string, associationRawData *model.InstanceAssociation) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// add or replace the cache
	c.set(associationID, associationRawData)
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *Cache) Get(associationID string) *model.InstanceAssociation {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	rawData, found := c.associaions[associationID]
	if !found {
		return nil
	}
	return rawData
}

// get gets an item from cache
func (c *Cache) get(associationId string) *model.InstanceAssociation {
	rawData, found := c.associaions[associationId]
	if !found {
		return nil
	}
	return rawData
}

// evict deletes the target entry from map
func (c *Cache) evict(associationID string) {
	delete(c.associaions, associationID)
}

// ValidateCache validates the current cache is not expired
func ValidateCache(rawData *model.InstanceAssociation) {

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	associationID := rawData.Association.AssociationId
	cached := cache.isCached(*associationID)
	if !cached {
		return
	}

	cachedRawData := cache.get(*associationID)
	newChecksum := *(rawData.Association.Checksum)
	cachedChecksum := *(cachedRawData.Association.Checksum)
	if newChecksum == cachedChecksum {
		return
	}
	// if checksum changes, evict the cached item
	cache.evict(*associationID)
}

// IsCached checks if the target cache exists
func (c *Cache) IsCached(associationID string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	_, found := c.associaions[associationID]
	return found
}

// isCached checks if the target cache exists
func (c *Cache) isCached(associationID string) bool {
	_, found := c.associaions[associationID]
	return found
}
