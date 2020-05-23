package cache

import (
	"sync"
    "time"
    "log"
)

type Users struct {
	sync.RWMutex
    items           map[string]User
}

type User struct {
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

func NewCacheUsers() *Users {

    cache := Users{
        items: make(map[string]User),
    }

    return &cache
}

func (c *Users) Set(key string, value User) {

    c.Lock()
    defer c.Unlock()

    c.items[key] = value

}

func (c *Users) Get(key string) (User, bool) {

    c.RLock()
    defer c.RUnlock()

    item, found := c.items[key]

    if !found {
        return User{}, false
    }

    return item, true
}

func (c *Users) Delete(key string) {

    c.Lock()
    defer c.Unlock()

    if _, found := c.items[key]; !found {
        log.Printf("[error] key not found in cache (%s)", key)
        return
    }

    delete(c.items, key)

}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *Users) Items() map[string]User {

	c.RLock()
    defer c.RUnlock()
    
	items := make(map[string]User, len(c.items))
	for k, v := range c.items {
		items[k] = v
    }
    
	return items
}

//cleaning cache items
func (c *Users) ClearItems(items map[string]User) {

    c.Lock()
    defer c.Unlock()

    for k, _ := range items {
        delete(c.items, k)
    }
}

func (c *Users) ExpiredItems() map[string]User {

    c.RLock()
    defer c.RUnlock()

    items := make(map[string]User)

    for k, v := range c.items {
        if time.Now().UTC().Unix() > v.EndsAt + 1800 {
            items[k] = v
        }
    }

    return items
}
