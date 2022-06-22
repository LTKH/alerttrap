package config

import (
    "fmt"
    "path"
    "errors"
    "regexp"
    "io/ioutil"
    "gopkg.in/yaml.v2"
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
    CertFile         string             `yaml:"cert_file"`
    CertKey          string             `yaml:"cert_key"`
    AlertsLimit      int                `yaml:"alerts_limit"`
    AlertsResolve    int64              `yaml:"alerts_resolve"`
    AlertsDelete     int64              `yaml:"alerts_delete"`
    SyncNodes        []string           `yaml:"sync_nodes"`
    DB               *DB                `yaml:"db"`
    Security         *Security          `yaml:"security"`
    Auth             *Auth              `yaml:"auth"`
    WebDir           string             `yaml:"web_dir"`
}

type Auth struct {
    Ldap             *Ldap              `yaml:"ldap"`
}

type Security struct {
    AdminUser        string             `yaml:"admin_user"`
    AdminPassword    string             `yaml:"admin_password"`
}

type DB struct {
    Client           string             `yaml:"client"`
    ConnString       string             `yaml:"conn_string"`
    HistoryDays      int                `yaml:"history_days"`
}

type Ldap struct {
    Enabled          bool               `yaml:"enabled"`
    SearchBase       string             `yaml:"search_base"`
    Host             string             `yaml:"host"`
    Port             int                `yaml:"port"`
    UseSsl           bool               `yaml:"use_ssl"`
    BindDn           string             `yaml:"bind_dn"`
    BindUser         string             `yaml:"bind_user"`
    BindPass         string             `yaml:"bind_pass"`
    UserFilter       string             `yaml:"user_filter"`
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
    Labels          map[string]string   `yaml:"labels"`
    Matchers        [][]Matcher
}

// Matcher models the matching of a label.
type Matcher struct {
    Type  string
    Name  string
    Value string
    Re *regexp.Regexp
}

// NewMatcher returns a matcher object.
func newMatcher(t, n, v string) (Matcher, error) {

    m := Matcher{
        Type:  t,
        Name:  n,
        Value: v,
    }

    if t != "=" && t != "!=" && t != "=~" && t != "!~" {
        return m, errors.New(fmt.Sprintf("executing query: invalid comparison operator: %s", t))
    }
    
    if t == "=~" || t == "!~" {
        re, err := regexp.Compile("^(?:" + v + ")$")
        if err != nil {
            return m, err
        }
        m.Re = re
    }

    return m, nil
}

func ParseMetricSelector(input string) (m []Matcher, err error) {
    var matchers []Matcher

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
