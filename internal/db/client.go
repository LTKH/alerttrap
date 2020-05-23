package db

import (
	"errors"
	"github.com/ltkh/alertstrap/internal/config"
    "github.com/ltkh/alertstrap/internal/cache"
	"github.com/ltkh/alertstrap/internal/db/mysql"
)

type DbClient interface {
	LoadAlerts() ([]cache.Alert, error)
	SaveAlerts(alerts map[string]cache.Alert) error
	AddAlert(alert cache.Alert) error
	UpdAlert(alert cache.Alert) error
	Close()
}

func NewClient(config *config.Config) (DbClient, error) {
	switch config.Db.Client {
	    case "mysql":
            return mysql.NewClient(config.Db.Conn_string)
	}
	return nil, errors.New("invalid client")
}