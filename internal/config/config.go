package config

import (
	"os"
	"github.com/naoina/toml"
)

type Config struct {
	DB               DB
	Ldap             Ldap
	Server           Server
	Menu             Menu
	Monit struct {
		Listen       string
	}
}

type DB struct {
	Client           string
	Conn_string      string
	History_days     int
	Alerts_table     string
	Users_table      string
}

type Ldap struct {
	Search_base      string
	Host             string
	Port             int
	Use_ssl          bool
	Bind_dn          string
	Bind_user        string
	Bind_pass        string
	User_filter      string
	Attributes       map[string]string
}

type Menu []struct {
	Text             string      `json:"text"`
	Type             string      `json:"-"`
	Href             string      `json:"href"`
	Nodes            []Node      `json:"nodes,omitempty"`
}

type Node struct {         
	Text             string      `json:"text"`
	Href             string      `json:"href"`
	Nodes            []Node      `json:"nodes,omitempty"`
}

type Server struct {
	Listen           string
	Cert_file        string
	Cert_key         string
	Alerts_limit     int
	Alerts_resolve   int64
	Alerts_delete    int64
	Log_max_size     int
	Log_max_backups  int
	Log_max_age      int
	Log_compress     bool
}

func New(filename string) (cfg Config, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	return cfg, toml.NewDecoder(f).Decode(&cfg)
}
