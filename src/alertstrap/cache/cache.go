package cache

import (
	  "sync"
      "time"
      "log"
)

type Cache struct {
    sync.RWMutex
    cleanupInterval time.Duration
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

func New(cleanupInterval time.Duration) *Cache {

    // инициализируем карту(map) в паре ключ(string)/значение(Item)
    items := make(map[string]Item)

    cache := Cache{
        cleanupInterval: cleanupInterval,
        items: items,
    }

    // Если интервал очистки больше 0, запускаем GC (удаление устаревших элементов)
    if cleanupInterval > 0 {
        cache.StartGC() // данный метод рассматривается ниже
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

func (c *Cache) StartGC()  {
    go c.GC()
}

func (c *Cache) GC() {

    for {

        // ожидаем время установленное в cleanupInterval
        //<-time.After(c.cleanupInterval)
        time.Sleep(c.cleanupInterval)

        if c.items != nil {
            // Ищем элементы с истекшим временем жизни и удаляем из хранилища
            for k, i := range c.items {
                if i.Expiration > 0 && time.Now().UTC().Unix() > i.Expiration {
                    delete(c.items, k)
                    log.Printf("[info] deleted cache index (%s)", k)
                }
            }
        }

    }

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
