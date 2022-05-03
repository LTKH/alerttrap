package cache

import (
	"sync"
    "time"
)

type Users struct {
	sync.RWMutex
    items            map[string]User
}

type User struct {
    Login            string             `yaml:"login" json:"login"`
    Email            string             `yaml:"email" json:"email"`
    Name             string             `yaml:"name" json:"name"`
    Password         string             `yaml:"password" json:"-"`
    Token            string             `yaml:"token" json:"token"`
    EndsAt           int64              `yaml:"endsAt" json:"-"`
}

func NewCacheUsers() *Users {

    cache := Users{
        items: make(map[string]User),
    }

    return &cache
}

func (u *Users) Set(key string, value User) {

    u.Lock()
    defer u.Unlock()

    u.items[key] = value

}

func (u *Users) Get(key string) (User, bool) {

    u.RLock()
    defer u.RUnlock()

    item, found := u.items[key]

    if !found {
        return User{}, false
    }

    return item, true
}

func (u *Users) Items() map[string]User {

	u.RLock()
    defer u.RUnlock()
    
	items := make(map[string]User, len(u.items))
	for k, v := range u.items {
		items[k] = v
    }
    
	return items
}

func (u *Users) ClearItems(items map[string]User) {

    u.Lock()
    defer u.Unlock()

    for k, _ := range items {
        delete(u.items, k)
    }
}

func (u *Users) ExpiredItems() map[string]User {

    u.RLock()
    defer u.RUnlock()

    items := make(map[string]User)

    for k, v := range u.items {
        if v.EndsAt > 0 {
            if time.Now().UTC().Unix() > v.EndsAt + 3600 {
                items[k] = v
            }
        }
    }

    return items
}
