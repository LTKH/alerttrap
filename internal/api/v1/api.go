package v1

import (
    "io"
    "net/http"
    "net/http/httputil"
    "log"
    "fmt"
    "os"
    "crypto/sha1"
    "encoding/hex"
    "net/url"
    "time"
    "sync"
    "strconv"
    "errors"
    "regexp"
    "io/ioutil"
    "encoding/json"
    "github.com/ltkh/alerttrap/internal/cache"
    "github.com/ltkh/alerttrap/internal/config"
    "github.com/ltkh/alerttrap/internal/ldap"
    "github.com/gorilla/websocket"
)

var (
    CacheAlerts *cache.Alerts = cache.NewCacheAlerts()
    CacheUsers *cache.Users = cache.NewCacheUsers()
    ConfigMenu = &Menu{}
    ConfigTmpl = &Tmpl{}
    reverseProxy *httputil.ReverseProxy
    reverseProxyOnce sync.Once

    upgrader = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin:     func(r *http.Request) bool { return true },
    }
)

type Api struct {
    Conf         *config.Config
    ProxyLog     chan *config.Proxy
}

type Change struct {
    Timestamp    int64                     `json:"timestamp"`
    Metrics      Metrics                   `json:"metrics"`
    Alerts       map[string][]Alert        `json:"alerts"`
}

type Metrics struct {
    AlertsCount  int                       `json:"alerttrap_alerts_count"`
    ChanCount    int                       `json:"alerttrap_chan_count"`
}

type Menu struct {
    sync.RWMutex
    items        []*config.Node
}

type Tmpl struct {
    sync.RWMutex
    items        []*config.Tmpl
}

type Resp struct {
    Status       string                    `json:"status"`
    Error        string                    `json:"error,omitempty"`
    Warnings     []string                  `json:"warnings,omitempty"`
    Data         interface{}               `json:"data"`
}

type Alerts struct {
    Position     int64                     `json:"position"`
    AlertsArray  []Alert                   `json:"alerts"`
}

type Alert struct {
    AlertId      string                    `json:"alertId"`
    GroupId      string                    `json:"groupId"`
    State        string                    `json:"state"`
    Status       string                    `json:"status,omitempty"`
    StartsAt     time.Time                 `json:"startsAt"`
    EndsAt       time.Time                 `json:"endsAt"`
    Repeat       int                       `json:"repeat"`
    ChangeSt     int                       `json:"changeSt"`
    Labels       map[string]interface{}    `json:"labels"`
    Annotations  map[string]interface{}    `json:"annotations"`
    GeneratorURL string                    `json:"generatorURL"`
}

func initReverseProxy() {
    reverseProxy = &httputil.ReverseProxy{
        Director: func(r *http.Request) {
            targetURL := r.Header.Get("proxy-target-url")
            target, err := url.Parse(targetURL)
            if err != nil {
                log.Printf("[error] unexpected error when parsing targetURL=%q: %s", targetURL, err)
                return
            }
            target.Path = r.URL.Path
            target.RawQuery = r.URL.RawQuery
            r.URL = target
        },
        Transport: func() *http.Transport {
            tr := http.DefaultTransport.(*http.Transport).Clone()
            tr.DisableCompression = true
            tr.ForceAttemptHTTP2 = false
            tr.MaxIdleConnsPerHost = 100
            if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
                tr.MaxIdleConns = tr.MaxIdleConnsPerHost
            }
            return tr
        }(),
        FlushInterval: time.Second,
        //ErrorLog:      logger.StdErrorLogger(),
        //ErrorLog:      log.New(new(bytes.Buffer), "", 0),
    }
}

func getReverseProxy() *httputil.ReverseProxy {
    reverseProxyOnce.Do(initReverseProxy)
    return reverseProxy
}

func encodeResp(resp *Resp) []byte {
    jsn, err := json.Marshal(resp)
    if err != nil {
        return encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)})
    }
    return jsn
}

func getHash(text string) string {
    h := sha1.New()
    io.WriteString(h, text)
    return hex.EncodeToString(h.Sum(nil))
}

// Matches returns whether the matcher matches the given string value.
func matches(m config.Matcher, s string) bool {
    switch m.Type {
        case "=":
            return s == m.Value
        case "!=":
            return s != m.Value
        case "=~":
            return m.Re.MatchString(s)
        case "!~":
            return !m.Re.MatchString(s)
    }
    return false
}

func checkLabels(labels map[string]interface{}, matchers [][]config.Matcher) bool {
    for _, mtch := range matchers {
        match := true

        for _, m := range mtch {
            if val, ok := labels[m.Name]; ok {
                if !matches(m, fmt.Sprintf("%v", val)) {
                    match = false
                    break
                }
            } else {
                if !matches(m, "") {
                    match = false
                    break
                }
            }
        }

        if match {
            return true
        }
    }

    return false
}

func getRules(nodes []*config.Node) (map[string]config.MatchingRule) {
    rules := map[string]config.MatchingRule{}

    for _, n := range nodes {
        if n.Href != "" {
            rules[n.Path] = n.MatchRules
        }
        for k, value := range getRules(n.Nodes) {
            rules[k] = value
        }
    }

    return rules
}

func checkMatch(a cache.Alert, r config.MatchingRule) bool {

    if r.IntArgs["position"] != 0 && r.IntArgs["position"] > a.ActiveAt {
        return false
    }
    if r.IntArgs["repeat_min"] != 0 && r.IntArgs["repeat_min"] > int64(a.Repeat) {
        return false
    }
    if r.IntArgs["repeat_max"] != 0 && r.IntArgs["repeat_max"] < int64(a.Repeat) {
        return false
    }
    if r.StrArgs["alert_id"] != "" && r.StrArgs["alert_id"] != a.AlertId {
        return false
    }
    if r.StrArgs["group_id"] != "" && r.StrArgs["group_id"] != a.GroupId {
        return false
    }
    if len(r.State) != 0 && r.State[a.State] == 0 {
        return false
    }
    if len(r.Matchers) != 0 && !checkLabels(a.Labels, r.Matchers) {
        return false
    }

    return true
}

func (m *Menu) Set(menu []*config.Node) error {
    m.Lock()
    defer m.Unlock()
    m.items = menu
    return nil
}

func (m *Menu) Get() ([]*config.Node, error) {
    m.RLock()
    defer m.RUnlock()
    return m.items, nil
}

func (t *Tmpl) Set(tmpl []*config.Tmpl) error {
    t.Lock()
    defer t.Unlock()
    t.items = tmpl
    return nil
}

func (t *Tmpl) Get() ([]*config.Tmpl, error) {
    t.RLock()
    defer t.RUnlock()
    return t.items, nil
}

func New(conf *config.Config) (*Api, error) {

    if conf.Global.Security.AdminUser != "" && conf.Global.Security.AdminPassword != "" {
        user := cache.User{
            Login:    conf.Global.Security.AdminUser,
            Password: getHash(conf.Global.Security.AdminPassword),
            Token:    getHash(string(time.Now().UTC().Unix())),
            Name:     conf.Global.Security.AdminUser,
            EndsAt:   0,
        }
        CacheUsers.Set(conf.Global.Security.AdminUser, user)
    }

    ConfigMenu.Set(conf.Menu)
    ConfigTmpl.Set(conf.Templates)
    
    return &Api{ Conf: conf, ProxyLog: make(chan *config.Proxy, 10000) }, nil
}

func (api *Api) Authentication(username, password string, r *http.Request) (cache.User, int, error) {

    if username != "" && password != "" {
        user, ok := CacheUsers.Get(username)
        if ok { 
            if user.Password == getHash(password) {
                return user, 204, nil
            }
        }
        return cache.User{}, 403, errors.New("Forbidden")
    }

    username, password, ok := r.BasicAuth()
    if ok {
        user, ok := CacheUsers.Get(username)
        if ok { 
            if user.Password == getHash(password) {
                return user, 204, nil
            }
        }
        return cache.User{}, 403, errors.New("Forbidden")
    }

    login, err := r.Cookie("login")
    if err != nil {
        return cache.User{}, 401, errors.New("Unauthorized")
    }
    token, err := r.Cookie("token")
    if err != nil {
        return cache.User{}, 401, errors.New("Unauthorized")
    }
    if login.Value != "" && token.Value != "" {
        user, ok := CacheUsers.Get(login.Value)
        if ok { 
            if user.Token == token.Value {
                return user, 204, nil
            }
        }
        return cache.User{}, 403, errors.New("Forbidden")
    }

    return cache.User{}, 401, errors.New("Unauthorized")
}

func (api *Api) ApiHealthy(w http.ResponseWriter, r *http.Request) {
    //alerts := []string{}

    //if err := api.Client.Healthy(); err != nil {
    //    alerts = append(alerts, err.Error())
    //}

    //if len(alerts) > 0 {
    //    w.WriteHeader(200)
    //    w.Write(encodeResp(&Resp{Status:"success", Warnings:alerts, Data:make(map[string]string, 0)}))
    //    return
    //}

    w.WriteHeader(200)
    w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiAuth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    user, code, err := api.Authentication("", "", r)
    if err != nil {
        w.WriteHeader(code)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    w.WriteHeader(200)
    w.Write(encodeResp(&Resp{Status:"success", Data:user}))
}

func (api *Api) ApiMenu(w http.ResponseWriter, r *http.Request) {
    _, code, err := api.Authentication("", "", r)
    if err != nil {
        w.WriteHeader(code)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    w.Header().Set("Content-Type", "application/json")
    menu, _ := ConfigMenu.Get()
    w.Write(encodeResp(&Resp{Status:"success", Data:menu}))
}

func (api *Api) ApiTmpl(w http.ResponseWriter, r *http.Request) {
    _, code, err := api.Authentication("", "", r)
    if err != nil {
        w.WriteHeader(code)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    w.Header().Set("Content-Type", "application/json")
    tmpl, _ := ConfigTmpl.Get()
    w.Write(encodeResp(&Resp{Status:"success", Data:tmpl}))
}

func (api *Api) ApiIndex(w http.ResponseWriter, r *http.Request){
    match, _ := regexp.MatchString("^/(|[a-z0-9]+.html|assets/.*)$", r.URL.Path)

    if !match {
        user, code, err := api.Authentication("", "", r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }
    
        if r.Header.Get("X-Custom-URL") != "" {
            if len(api.ProxyLog) < 10000 {
                api.ProxyLog <- &config.Proxy{
                    Login:     user.Login,
                    Method:    r.Method,
                    Url:       r.Header.Get("X-Custom-URL"),
                    Path:      r.URL.Path,
                    Timestamp: time.Now().UTC().Unix(),
                }
            }

            r.Header.Set("proxy-target-url", r.Header.Get("X-Custom-URL"))
            getReverseProxy().ServeHTTP(w, r)
            return
        }
    }

    if _, err := os.Stat(api.Conf.Global.WebDir+r.URL.Path); err == nil {
        http.ServeFile(w, r, api.Conf.Global.WebDir+r.URL.Path)
    } else {
        index, _ := os.ReadFile(api.Conf.Global.WebDir+"/index.html")
        w.WriteHeader(404)
        w.Write(index)
    }
}

func addAlert(id string, alert cache.Alert) {

    CacheAlerts.Set(id, alert)
}

func (api *Api) SetAlerts(data Alerts) {
    for _, value := range data.AlertsArray {

        for _, ext := range api.Conf.ExtensionRules {
            for _, mrs := range ext.Matchers {
                matchers := [][]config.Matcher{ mrs }
                if checkLabels(value.Labels, matchers) {
                    for k, v := range ext.Labels {
                        value.Labels[k] = v
                    }
                }
            }
        }

        labels, err := json.Marshal(value.Labels)
        if err != nil {
            log.Printf("[error] read alert %v", err)
            return
        }

        group_id := getHash(string(labels))

        if value.Status != "resolved" {
            if value.Labels["severity"] != nil {
                value.State = value.Labels["severity"].(string)
            }
        } else {
            value.State = value.Status
        }
    
        starts_at := value.StartsAt.UTC().Unix()
        ends_at   := value.EndsAt.UTC().Unix()
        if starts_at < 0 {
            starts_at  = time.Now().UTC().Unix()
        } 
        if ends_at < 0 {
            ends_at    = time.Now().UTC().Unix() + api.Conf.Global.AlertsResolve
        } 

        alert, found := CacheAlerts.Get(group_id)
        if found {

            if alert.State != value.State {
                alert.ChangeSt ++ 
            }

            alert.State          = value.State
            alert.ActiveAt       = time.Now().UTC().Unix()
            alert.EndsAt         = ends_at
            alert.Annotations    = value.Annotations
            alert.GeneratorURL   = value.GeneratorURL
            alert.Repeat         = alert.Repeat + 1

            addAlert(group_id, alert)
    
        } else {

            alert_id := getHash(string(strconv.FormatInt(time.Now().UTC().UnixNano(), 16)+group_id))
            
            alert := cache.Alert{}

            alert.AlertId        = alert_id
            alert.GroupId        = group_id
            alert.State          = value.State
            alert.ActiveAt       = time.Now().UTC().Unix()
            alert.StartsAt       = starts_at
            alert.EndsAt         = ends_at
            alert.Labels         = value.Labels
            alert.Annotations    = value.Annotations
            alert.GeneratorURL   = value.GeneratorURL
            alert.Repeat         = 1
            alert.ChangeSt       = 0
    
            addAlert(group_id, alert)

        }

    }
}

func (api *Api) ApiAlerts(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    if r.Method == "GET" {

        _, code, err := api.Authentication("", "", r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        matchRule, err := config.ParseQueryValues(r.URL.Query())
        if err != nil {
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        alerts := Alerts{}

        for _, a := range CacheAlerts.Items() {

            if !checkMatch(a, matchRule) {
                continue
            }

            alert := Alert{}

            alert.AlertId      = a.AlertId
            alert.GroupId      = a.GroupId
            alert.State        = a.State
            alert.StartsAt     = time.Unix(a.StartsAt, 0)
            alert.EndsAt       = time.Unix(a.EndsAt, 0)
            alert.Repeat       = a.Repeat
            alert.ChangeSt     = a.ChangeSt
            alert.Labels       = a.Labels
            alert.Annotations  = a.Annotations
            alert.GeneratorURL = a.GeneratorURL

            alerts.AlertsArray = append(alerts.AlertsArray, alert)

            if a.ActiveAt > alerts.Position {
                alerts.Position  = a.ActiveAt
            }
            
            if len(alerts.AlertsArray) >= matchRule.Limit {
                continue
            }
        }

        if len(alerts.AlertsArray) == 0 {
            alerts.AlertsArray = make([]Alert, 0)
        } else {
            if matchRule.Limit == api.Conf.Global.AlertsLimit {
                warnings := []string{}
                warnings = append(warnings, fmt.Sprintf("display limit exceeded - %d", matchRule.Limit))
                w.Write(encodeResp(&Resp{Status:"success", Warnings:warnings, Data:alerts}))
                return
            }
        }
        
        w.Write(encodeResp(&Resp{Status:"success", Data:alerts}))
        return
    }

    if r.Method == "DELETE" {

        _, code, err := api.Authentication("", "", r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        if r.URL.Query()["group_id"] != nil {
            
            _, found := CacheAlerts.Get(r.URL.Query()["group_id"][0])
            if found {
                CacheAlerts.Delete(r.URL.Query()["group_id"][0])
                w.WriteHeader(200)
                w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
                return
            }

            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:"Alert Not Found", Data:make(map[string]string, 0)}))
            return

        }

        w.WriteHeader(400)
        w.Write(encodeResp(&Resp{Status:"error", Error:"group_id required", Data:make(map[string]string, 0)}))
        return
    }

    if r.Method == "POST" {

        alerts := Alerts{}

        body, err := ioutil.ReadAll(r.Body)
        if err != nil {
            log.Printf("[error] %v - %s", err, r.URL.Path)
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        if err := json.Unmarshal(body, &alerts); err != nil {
            log.Printf("[error] %v - %s", err, r.URL.Path)
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        go api.SetAlerts(alerts)

        w.WriteHeader(204)
        return
    }

    w.WriteHeader(405)
    w.Write(encodeResp(&Resp{Status:"error", Error:"Method Not Allowed", Data:make(map[string]string, 0)}))
}

func (api *Api) Api2Alerts(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    if r.Method == "GET" {

        _, code, err := api.Authentication("", "", r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        matchRule, err := config.ParseQueryValues(r.URL.Query())
        if err != nil {
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        alerts := Alerts{}

        for _, a := range CacheAlerts.Items() {

            if !checkMatch(a, matchRule) {
                continue
            }

            alert := Alert{}

            alert.AlertId      = a.AlertId
            alert.GroupId      = a.GroupId
            alert.State        = a.State
            alert.StartsAt     = time.Unix(a.StartsAt, 0)
            alert.EndsAt       = time.Unix(a.EndsAt, 0)
            alert.Repeat       = a.Repeat
            alert.ChangeSt     = a.ChangeSt
            alert.Labels       = a.Labels
            alert.Annotations  = a.Annotations
            alert.GeneratorURL = a.GeneratorURL

            alerts.AlertsArray = append(alerts.AlertsArray, alert)

            if a.ActiveAt > alerts.Position {
                alerts.Position  = a.ActiveAt
            }
            
            if len(alerts.AlertsArray) >= matchRule.Limit {
                continue
            }
        }

        if len(alerts.AlertsArray) == 0 {
            alerts.AlertsArray = make([]Alert, 0)
        } else {
            if matchRule.Limit == api.Conf.Global.AlertsLimit {
                warnings := []string{}
                warnings = append(warnings, fmt.Sprintf("display limit exceeded - %d", matchRule.Limit))
                w.Write(encodeResp(&Resp{Status:"success", Warnings:warnings, Data:alerts}))
                return
            }
        }
        
        w.Write(encodeResp(&Resp{Status:"success", Data:alerts}))
        return
    }

    if r.Method == "DELETE" {

        _, code, err := api.Authentication("", "", r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        if r.URL.Query()["group_id"] != nil {
            
            _, found := CacheAlerts.Get(r.URL.Query()["group_id"][0])
            if found {
                CacheAlerts.Delete(r.URL.Query()["group_id"][0])
                w.WriteHeader(200)
                w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
                return
            }

            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:"Alert Not Found", Data:make(map[string]string, 0)}))
            return

        }

        w.WriteHeader(400)
        w.Write(encodeResp(&Resp{Status:"error", Error:"group_id required", Data:make(map[string]string, 0)}))
        return
    }

    if r.Method == "POST" {

        alerts := []Alert{}

        body, err := ioutil.ReadAll(r.Body)
        if err != nil {
            log.Printf("[error] %v - %s", err, r.URL.Path)
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        if err := json.Unmarshal(body, &alerts); err != nil {
            log.Printf("[error] %v - %s", err, r.URL.Path)
            w.WriteHeader(400)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        go api.SetAlerts(Alerts{AlertsArray: alerts})

        w.WriteHeader(200)
        w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
        return
    }

    w.WriteHeader(405)
    w.Write(encodeResp(&Resp{Status:"error", Error:"Method Not Allowed", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiLogin(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    err := r.ParseForm()
    if err != nil {
        w.WriteHeader(400)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    username := r.Form.Get("username")
    password := r.Form.Get("password")

    if username == "" || password == "" {
        w.WriteHeader(403)
        w.Write(encodeResp(&Resp{Status:"error", Error:"Login or password is empty", Data:make(map[string]string, 0)}))
        return
    }

    if username == api.Conf.Global.Security.AdminUser {
        user, code, err := api.Authentication(username, password, r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        w.WriteHeader(200)
        w.Write(encodeResp(&Resp{Status:"success", Data:user}))
        return
    }

    if api.Conf.Global.Auth.Ldap.Enabled {
        if api.Conf.Global.Auth.Ldap.BindUser == "" && api.Conf.Global.Auth.Ldap.BindPass == "" {
            api.Conf.Global.Auth.Ldap.BindUser = username
            api.Conf.Global.Auth.Ldap.BindPass = password
        }

        attributes := []string{}
        for _, val := range api.Conf.Global.Auth.Ldap.Attributes {
            attributes = append(attributes, val)
        }

        clnt := &ldap.LDAPClient{
            Base:         api.Conf.Global.Auth.Ldap.SearchBase,
            Host:         api.Conf.Global.Auth.Ldap.Host,
            Port:         api.Conf.Global.Auth.Ldap.Port,
            UseSSL:       api.Conf.Global.Auth.Ldap.UseSsl,
            BindDN:       fmt.Sprintf(api.Conf.Global.Auth.Ldap.BindDn, api.Conf.Global.Auth.Ldap.BindUser),
            BindPassword: api.Conf.Global.Auth.Ldap.BindPass,
            UserFilter:   api.Conf.Global.Auth.Ldap.UserFilter,
            Attributes:   attributes,
        }
        defer clnt.Close()

        ok, usr, err := clnt.Authenticate(username, password)
        if !ok {
            log.Printf("[error] user authenticating %s: %+v", username, err)
            w.WriteHeader(403)
            w.Write(encodeResp(&Resp{Status:"error", Error:"See application log for more details", Data:make(map[string]string, 0)}))
            return
        }

        user := cache.User{}
        user.Login = username
        user.Password = getHash(password)
        user.Token = getHash(string(time.Now().UTC().Unix()))
        if api.Conf.Global.Auth.Ldap.Attributes["name"] != "" {
            user.Name = usr[api.Conf.Global.Auth.Ldap.Attributes["name"]]
        }
        if api.Conf.Global.Auth.Ldap.Attributes["email"] != "" {
            user.Email = usr[api.Conf.Global.Auth.Ldap.Attributes["email"]]
        }
        
        CacheUsers.Set(username, user)

        w.WriteHeader(200)
        w.Write(encodeResp(&Resp{Status:"success", Data:user}))
        return
    }

    log.Printf("[error] user authenticating %s", username)
    w.WriteHeader(403)
    w.Write(encodeResp(&Resp{Status:"error", Error:"Invalid username or password", Data:make(map[string]string, 0)}))
}
