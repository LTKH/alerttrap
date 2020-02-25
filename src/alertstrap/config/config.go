package config

import (
  "time"
  "os"
  "github.com/naoina/toml"
)

type Config struct {
  Mysql struct {
    Conn_string  string
  }
  Alertstrap struct {
    Listen_port  string
    Login        string
    Passwd       string
    Location     string
  }
  Jiramanager struct {
    Tmpl_dir     string
    Get_alerts   string
    Jira_api     string
    Login        string
    Passwd       string
    Search       bool
    Interval     time.Duration
    Debug        bool
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
