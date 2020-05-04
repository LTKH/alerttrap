package config

import (
  //"time"
  "os"
  "github.com/naoina/toml"
)

type Config struct {
  Mysql struct {
    Conn_string  string
  }
  Server struct {
    Listen       string
    Cert_file    string
    Cert_key     string
  }
  Monit struct {
		Listen       string
	}
}

func LoadConfigFile(filename string) (cfg Config, err error) {
  f, err := os.Open(filename)
  if err != nil {
    return cfg, err
  }
  defer f.Close()

  return cfg, toml.NewDecoder(f).Decode(&cfg)
}
