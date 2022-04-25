package config

import (
    "fmt"
    //"log"
    "path"
    "errors"
    "regexp"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "github.com/ltkh/alerttrap/internal/cache"
)

var (
    ReLabels = regexp.MustCompile(`(?:([\w]+)([=!~><]{1,2})"([^"]*)")`)
)

type Config struct {
    Global           *Global            `yaml:"global"`
    Menu             []*Node            `yaml:"menu"`
    ExtensionRules   []*ExtensionRule   `yaml:"extension_rules"`
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

type ExtensionRule struct {
    SourceMatchers  []string            `yaml:"source_matchers"`
    Labels          []map[string]string `yaml:"labels"`
    Matchers        [][]*Matcher
}

// Matcher models the matching of a label.
type Matcher struct {
    Type  string
    Name  string
    Value string
    Re *regexp.Regexp
}

// NewMatcher returns a matcher object.
func newMatcher(t, n, v string) (*Matcher, error) {

    if t != "=" && t != "!=" && t != "=~" && t != "!~" {
        return nil, errors.New(fmt.Sprintf("executing query: invalid comparison operator: %s", t))
    }

    m := &Matcher{
        Type:  t,
        Name:  n,
        Value: v,
    }
    
    if t == "=~" || t == "!~" {
        re, err := regexp.Compile("^(?:" + v + ")$")
        if err != nil {
            return nil, err
        }
        m.Re = re
    }

    return m, nil
}

func ParseMetricSelector(input string) (m []*Matcher, err error) {
    var matchers []*Matcher

    lbls := ReLabels.FindAllStringSubmatch(input, -1)
    for _, l := range lbls {

        matcher, err := newMatcher(l[2], l[1], l[3])
        if err != nil {
            return nil, err
        }

        matchers = append(matchers, matcher)
    }

    return matchers, nil
}

func pathNodes(p string, nodes []*Node) {
    for _, n := range nodes {
        n.Path = path.Join(p, n.Id)
        pathNodes(n.Path, n.Nodes)
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

    pathNodes("/", cfg.Menu)

    for _, ext := range cfg.ExtensionRules {
        for _, sm := range ext.SourceMatchers {
            m, err := ParseMetricSelector(sm)
            if err != nil {
                return cfg, err
            }
            ext.Matchers = append(ext.Matchers, m)
        }
    }
    
    return cfg, nil
}
