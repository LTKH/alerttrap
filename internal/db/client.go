package db

import (
    "errors"
    "github.com/ltkh/alerttrap/internal/config"
    "github.com/ltkh/alerttrap/internal/cache"
    "github.com/ltkh/alerttrap/internal/db/mysql"
    "github.com/ltkh/alerttrap/internal/db/sqlite3"    
)

type DbClient interface {
    Close() error
    CreateTables() error
    Healthy() error
    LoadUser(login string) (cache.User, error)
    SaveUser(user cache.User) error
    LoadUsers(timestamp int64) ([]cache.User, error)
    LoadAlerts() ([]cache.Alert, error)
    SaveAlerts(alerts map[string]cache.Alert) error
    SaveAction(action config.Action) error
    AddAlert(alert cache.Alert) error
    UpdAlert(alert cache.Alert) error
    DeleteOldAlerts() (int64, error)
    DeleteOldActions() (int64, error)
}

func NewClient(config *config.DB) (DbClient, error) {
    switch config.Client {
        case "mysql":
            return mysql.NewClient(config)
        case "sqlite3":
            return sqlite3.NewClient(config)
    }
    return nil, errors.New("invalid client")
}