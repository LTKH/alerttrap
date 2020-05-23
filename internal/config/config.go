package config

import (
	//"time"
	"os"
	"github.com/naoina/toml"
)

type Config struct {
	Db struct {
		Client       string
		Conn_string  string
	}
	Alerts           Alerts
	Server struct {
		Listen       string
		Cert_file    string
		Cert_key     string
	}
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
