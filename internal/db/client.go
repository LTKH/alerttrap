package db

import (
	"errors"
	"github.com/ltkh/alertstrap/internal/config"
    "github.com/ltkh/alertstrap/internal/cache"
	"github.com/ltkh/alertstrap/internal/db/mysql"
)

type DbClient interface {
	Healthy() error
	LoadUser(login string) (cache.User, error)
	SaveUser(user cache.User) error
	LoadUsers() ([]cache.User, error)
	LoadAlerts() ([]cache.Alert, error)
	SaveAlerts(alerts map[string]cache.Alert) error
	AddAlert(alert cache.Alert) error
	UpdAlert(alert cache.Alert) error
	DeleteOldAlerts() (int64, error)
}

func NewClient(config *config.DB) (DbClient, error) {
	switch config.Client {
	    case "mysql":
            return mysql.NewClient(config)
	}
	return nil, errors.New("invalid client")
}