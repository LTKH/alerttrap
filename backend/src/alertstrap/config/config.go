package config

import (
  "time"
  "os"
  "github.com/naoina/toml"
)

type Config struct {
  Mysql struct {
    Conn_string  string
    Alerts_table string
    Alerts_view  string
    Hosts_table  string
    Tasks_table  string
  }
  Alertstrap struct {
    Listen_port  string
    Get_alerts   string
    Login        string
    Passwd       string
  }
  Jiramanager struct {
    Jira_api     string
    Interval     time.Duration
    Login        string
    Passwd       string
    Debug        bool
    Templates    string
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
