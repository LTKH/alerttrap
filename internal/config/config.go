package config

import (
    "path"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "github.com/ltkh/alerttrap/internal/cache"
)

type Config struct {
    Global           *Global            `yaml:"global"`
    Menu             []*Node            `yaml:"menu"`
}

type Global struct {
    Listen           string             `yaml:"listen_address"`
    Cert_file        string             `yaml:"cert_file"`
    Cert_key         string             `yaml:"cert_key"`
    Alerts_limit     int                `yaml:"alerts_limit"`
    Alerts_resolve   int64              `yaml:"alerts_resolve"`
    Alerts_delete    int64              `yaml:"alerts_delete"`
    Sync_nodes       []string           `yaml:"sync_nodes"`
    DB               *DB                `yaml:"db"`
    Users            *[]cache.User      `yaml:"users"`
    Ldap             *Ldap              `yaml:"ldap"`
    Monit            *Monit             `yaml:"monit"`
}

type Monit struct {
    Listen           string             `yaml:"listen_address"`
}

type DB struct {
    Client           string             `yaml:"client"`
    Conn_string      string             `yaml:"conn_string"`
    History_days     int                `yaml:"history_days"`
}

type Ldap struct {
    Search_base      string             `yaml:"search_base"`
    Host             string             `yaml:"host"`
    Port             int                `yaml:"port"`
    Use_ssl          bool               `yaml:"use_ssl"`
    Bind_dn          string             `yaml:"bind_dn"`
    Bind_user        string             `yaml:"bind_user"`
    Bind_pass        string             `yaml:"bind_pass"`
    User_filter      string             `yaml:"user_filter"`
    Attributes       map[string]string  `yaml:"attributes"`
}

type Node struct {   
    Id               string             `yaml:"id" json:"id"`      
    Name             string             `yaml:"name" json:"name"`
    Path             string             `yaml:"path" json:"path"`
    Href             string             `yaml:"href" json:"href"`
    Tags             []string           `yaml:"tags" json:"tags"`
    Options          map[string]string  `yaml:"options" json:"options,omitempty"`
    Class            string             `yaml:"class" json:"class,omitempty"`
    Summary          string             `yaml:"summary" json:"summary,omitempty"`
    Nodes            []*Node            `yaml:"nodes" json:"nodes,omitempty"`
}

func path_nodes(p string, nodes []*Node) {
    for _, n := range nodes {
        n.Path = path.Join(p, n.Id)
        path_nodes(n.Path, n.Nodes)
    }
}

func New(filename string) (*Config, error) {

    cfg := &Config{}

    content, err := ioutil.ReadFile(filename)
    if err != nil {
       return cfg, err
    }

    if err := yaml.UnmarshalStrict(content, cfg); err != nil {
        return cfg, err
    }

    path_nodes("/", cfg.Menu)
    
    return cfg, nil
}
