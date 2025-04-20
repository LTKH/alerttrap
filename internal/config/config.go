package config

import (
    "os"
    "fmt"
    "log"
    "path"
    "errors"
    "regexp"
    "strings"
    "strconv"
    "net/url"
    "io/ioutil"
    "gopkg.in/yaml.v2"
)

var (
    ReLabels = regexp.MustCompile(`(?:([\w]+)([=!~><]{1,2})"([^"]*)")`)
)

type Config struct {
    Global           *Global                 `yaml:"global"`
    Menu             []*Node                 `yaml:"menu"`
    Templates        []*Tmpl                 `yaml:"templates"`
    ExtensionRules   []*ExtensionRule        `yaml:"extension_rules"`
}

type Tmpl struct {
    UrlMatcher   string                      `yaml:"url_matcher" json:"url_matcher"`
    TargetPage   string                      `yaml:"target_page" json:"target_page"`
}

type Global struct {
    CertFile         string                  `yaml:"cert_file"`
    CertKey          string                  `yaml:"cert_key"`
    AlertsLimit      int                     `yaml:"alerts_limit"`
    AlertsResolve    int64                   `yaml:"alerts_resolve"`
    AlertsDelete     int64                   `yaml:"alerts_delete"`
    SyncNodes        []string                `yaml:"sync_nodes"`
    DB               *DB                     `yaml:"db"`
    Security         *Security               `yaml:"security"`
    Auth             *Auth                   `yaml:"auth"`
    WebDir           string                  `yaml:"web_dir"`
}

type Auth struct {
    Ldap             *Ldap                   `yaml:"ldap"`
}

type Security struct {
    AdminUser        string                  `yaml:"admin_user"`
    AdminPassword    string                  `yaml:"admin_password"`
}

type DB struct {
    Client           string                  `yaml:"client"`
    ConnString       string                  `yaml:"conn_string"`
    HistoryDays      int                     `yaml:"history_days"`
    Host             string                  `yaml:"host"`
    Name             string                  `yaml:"name"`
    User             string                  `yaml:"user"`
    Password         string                  `yaml:"password"`
}

type Ldap struct {
    Enabled          bool                    `yaml:"enabled"`
    SearchBase       string                  `yaml:"search_base"`
    Host             string                  `yaml:"host"`
    Port             int                     `yaml:"port"`
    UseSsl           bool                    `yaml:"use_ssl"`
    BindDn           string                  `yaml:"bind_dn"`
    BindUser         string                  `yaml:"bind_user"`
    BindPass         string                  `yaml:"bind_pass"`
    UserFilter       string                  `yaml:"user_filter"`
    Attributes       map[string]string       `yaml:"attributes"`
}

type Node struct {   
    Id               string                  `yaml:"id" json:"id"`      
    Name             string                  `yaml:"name" json:"name"`
    Path             string                  `yaml:"path" json:"path"`
    Href             string                  `yaml:"href" json:"href"`
    Tags             []string                `yaml:"tags" json:"tags"`
    Options          map[string]interface{}  `yaml:"options" json:"options,omitempty"`
    Class            string                  `yaml:"class" json:"class,omitempty"`
    Summary          string                  `yaml:"summary" json:"summary,omitempty"`
    Nodes            []*Node                 `yaml:"nodes" json:"nodes,omitempty"`
    MatchRules       MatchingRule            `yaml:"-" json:"-"`
}

type ExtensionRule struct {
    SourceMatchers   []string                 `yaml:"source_matchers"`
    Labels           map[string]string        `yaml:"labels"`
    Matchers         [][]Matcher
}

type MatchingRule struct {
    IntArgs          map[string]int64
    StrArgs          map[string]string
    Matchers         [][]Matcher
    State            map[string]int
    Limit            int
}

// Matcher models the matching of a label.
type Matcher struct {
    Type  string
    Name  string
    Value string
    Re *regexp.Regexp
}

type Proxy struct {
    Login         string
    Method        string
    Url           string
    Path          string
    Timestamp     int64
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

func ParseMetricSelector(input string) ([]Matcher, error) {
    matchers := []Matcher{}

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

func ParseQueryValues(values map[string][]string) (MatchingRule, error) {
    matchRule := MatchingRule{}

    for k, v := range values {
        switch k {
            case "alert_id","group_id":
                matchRule.StrArgs[k] = v[0]
            case "state":
                for _, st := range strings.Split(v[0], "|") {
                    matchRule.State[st] = 1
                }
            case "position","repeat_min","repeat_max":
                i, err := strconv.Atoi(v[0])
                if err != nil {
                    return matchRule, err
                }
                matchRule.IntArgs[k] = int64(i)
            case "limit":
                l, err := strconv.Atoi(v[0])
                if err != nil {
                    return matchRule, err
                }
                matchRule.Limit = l
            case "match[]":
                for _, s := range v {
                    mrs, err := ParseMetricSelector(s)
                    if err != nil {
                        return matchRule, err
                    }
                    matchRule.Matchers = append(matchRule.Matchers, mrs)
                }
            default:
                return matchRule, fmt.Errorf("executing query: invalid parameter '%v'", k)
        }
    }

    return matchRule, nil
}

func pathNodes(p string, nodes []*Node) error {
    for _, n := range nodes {
        u, err := url.Parse(n.Href)
        if err != nil {
            return err
        }

        m, err := url.ParseQuery(u.RawQuery)
        if err != nil {
            return err
        }

        n.MatchRules, err = ParseQueryValues(m)
        if err != nil {
            return err
        }

        n.Path = path.Join(p, n.Id)
        if err := pathNodes(n.Path, n.Nodes); err != nil {
            return err
        }
    }

    return nil
}

func getEnv(value string) string {
    if len(value) > 0 && string(value[0]) == "$" {
        val, ok := os.LookupEnv(strings.TrimPrefix(value, "$"))
        if !ok {
            log.Printf("[error] no value found for %v", value)
            return ""
        }
        return val
    }

    return value
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

    if err := pathNodes("/", cfg.Menu); err != nil {
        return cfg, err
    }

    cfg.Global.Security.AdminUser = getEnv(cfg.Global.Security.AdminUser)
    cfg.Global.Security.AdminPassword = getEnv(cfg.Global.Security.AdminPassword)
    cfg.Global.DB.User = getEnv(cfg.Global.DB.User)
    cfg.Global.DB.Password = getEnv(cfg.Global.DB.Password)
    

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
