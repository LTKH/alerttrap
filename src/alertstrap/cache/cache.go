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

    go func(c *Cache){

        time.Sleep(10 * time.Second)

        if c.items != nil {
            // Ищем элементы с истекшим временем жизни и удаляем из хранилища
            if keys := c.expiredKeys(); len(keys) != 0 {
                c.clearItems(keys)

            }
        }

    }(&cache)

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

// clearItems удаляет ключи из переданного списка, в нашем случае "просроченные"
func (c *Cache) clearItems(keys []string) {

    c.Lock()

    defer c.Unlock()

    for _, k := range keys {
        delete(c.items, k)
    }
}

func (c *Cache) expiredKeys() (keys []string) {

    c.RLock()

    defer c.RUnlock()

    for k, i := range c.items {
        if i.Expiration > 0 && time.Now().UTC().Unix() > i.Expiration {
            keys = append(keys, k)
        }
    }

    return
}
