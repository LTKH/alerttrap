package cache

import (
	"sync"
    "time"
    "log"
)

type Cache struct {
    sync.RWMutex
    items           map[string]Item
}

type Item struct {
    Value           Alert
    Expiration      int64
}

type Alert struct {
    AlertId         string                  `json:"alertId"`
    GroupId         string                  `json:"groupId"`
    Status          string                  `json:"status"`
    StartsAt        int64                   `json:"startsAt"`
    EndsAt          int64                   `json:"endsAt"`
    Duplicate       int                     `json:"duplicate"`
    Labels          map[string]interface{}  `json:"labels"`
    Annotations     map[string]interface{}  `json:"annotations"`
    GeneratorURL    string                  `json:"generatorURL"`
}

func New() *Cache {

    cache := Cache{
        items: make(map[string]Item),
    }

    return &cache
}

func (c *Cache) Set(key string, value Alert, expiration int64) {

    c.Lock()
    defer c.Unlock()

    c.items[key] = Item{
        Value:      value,
        Expiration: expiration,
    }

}

func (c *Cache) Get(key string) (Alert, bool) {

    c.RLock()
    defer c.RUnlock()

    item, found := c.items[key]

    if !found {
        return Alert{}, false
    }

    return item.Value, true
}

func (c *Cache) Delete(key string) {

    c.Lock()
    defer c.Unlock()

    if _, found := c.items[key]; !found {
        log.Printf("[error] key not found in cache (%s)", key)
        return
    }

    delete(c.items, key)

}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *Cache) Items() map[string]Item {

	c.RLock()
    defer c.RUnlock()
    
	items := make(map[string]Item, len(c.items))
	for k, v := range c.items {
		items[k] = v
    }
    
	return items
}

//cleaning cache items
func (c *Cache) ClearItems(items map[string]Item) {

    c.Lock()
    defer c.Unlock()

    for k, _ := range items {
        delete(c.items, k)
    }
}

func (c *Cache) ExpiredItems() map[string]Item {

    c.RLock()
    defer c.RUnlock()

    items := make(map[string]Item)

    for k, v := range c.items {
        if v.Expiration > 0 && time.Now().UTC().Unix() > v.Expiration {
            items[k] = v
        }
    }

    return items
}
