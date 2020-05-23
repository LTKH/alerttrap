package config

import (
	//"time"
	"os"
	"github.com/naoina/toml"
)

type Config struct {
	DB               DB
	Alerts           Alerts
	Server           Server
	Monit struct {
		Listen       string
	}
	Menu []struct {
		Name         string
		Type         string
		Section []struct {
			Name     string
			Url      string
		}
	}
}

type Server struct {
	Listen       string
	Cert_file    string
	Cert_key     string
}

type DB struct {
	Client       string
	Conn_string  string
	History_days int
}

type Alerts struct {
	Limit           int
	Resolve         int64
	Delete          int64
}

func New(filename string) (cfg Config, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	return cfg, toml.NewDecoder(f).Decode(&cfg)
}
