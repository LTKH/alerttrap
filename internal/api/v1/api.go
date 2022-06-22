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
    "strings"
    "io/ioutil"
    "encoding/json"
    "github.com/ltkh/alerttrap/internal/cache"
    "github.com/ltkh/alerttrap/internal/config"
    "github.com/ltkh/alerttrap/internal/ldap"
)

var (
    CacheAlerts *cache.Alerts = cache.NewCacheAlerts()
    CacheUsers *cache.Users = cache.NewCacheUsers()
    reverseProxy *httputil.ReverseProxy
	reverseProxyOnce sync.Once
)

type Api struct {
    Conf         *config.Config
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
            targetURL := r.Header.Get("X-Custom-Proxy")
            target, err := url.Parse(targetURL)
            if err != nil {
                log.Fatalf("[error] unexpected error when parsing targetURL=%q: %s", targetURL, err)
            }
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

func checkMatches(labels map[string]interface{}, matchers [][]config.Matcher) bool {
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

func authentication(cfg *config.DB, r *http.Request) (string, int, error) {

    login, password, ok := r.BasicAuth()
    if ok {
        user, ok := CacheUsers.Get(login)
        if ok { 
            if user.Password == getHash(password) {
                return login, 204, nil
            }
        }
        return login, 403, errors.New("Forbidden")
    }

    lg, err := r.Cookie("login")
    if err != nil {
        return "", 401, errors.New("Unauthorized")
    }
    tk, err := r.Cookie("token")
    if err != nil {
        return "", 401, errors.New("Unauthorized")
    }
    if lg.Value != "" && tk.Value != "" {
        user, ok := CacheUsers.Get(lg.Value)
        if ok { 
            if user.Token == tk.Value {
                return lg.Value, 204, nil
            }
        }
        return lg.Value, 403, errors.New("Forbidden")
    }

    return "", 401, errors.New("Unauthorized")
}

func New(conf *config.Config) (*Api, error) {
    //connection to data base
    //client, err := db.NewClient(conf.Global.DB)
    //if err != nil {
    //    return nil, err
    //}
    //log.Print("[info] connected to dbase")
    //loading users
    //users, err := client.LoadUsers()
    //if err != nil {
    //    return nil, err
    //}
    //for _, user := range users {
    //    CacheUsers.Set(user.Login, user)
    //}
    //log.Printf("[info] loaded users from dbase (%d)", len(users))

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
    
    return &Api{ Conf: conf }, nil
}

func (api *Api) ApiHealthy(w http.ResponseWriter, r *http.Request) {
    //var alerts []string

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

    login, code, err := authentication(api.Conf.Global.DB, r)
    if err != nil {
        w.WriteHeader(code)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    user, ok := CacheUsers.Get(login)
    if ok { 
        w.WriteHeader(200)
        w.Write(encodeResp(&Resp{Status:"success", Data:user}))
        return
    }

    w.WriteHeader(code)
    w.Write(encodeResp(&Resp{Status:"success", Data:make(map[string]string, 0)}))
}

func (api *Api) ApiMenu(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write(encodeResp(&Resp{Status:"success", Data:api.Conf.Menu}))
}

func (api *Api) ApiSync(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    var alert Alert

    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Printf("[error] %v - %s", err, r.URL.Path)
        w.WriteHeader(400)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }

    if err := json.Unmarshal(body, &alert); err != nil {
        log.Printf("[error] %v - %s", err, r.URL.Path)
        w.WriteHeader(400)
        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
        return
    }
}

func (api *Api) ApiIndex(w http.ResponseWriter, r *http.Request){
    targetURL := r.Header.Get("X-Custom-Proxy")
    if targetURL != "" {
        r.Header.Set("X-Custom-Proxy", targetURL)
        getReverseProxy().ServeHTTP(w, r)
        return
    }

    if _, err := os.Stat(api.Conf.Global.WebDir+r.URL.Path); err == nil {
        http.ServeFile(w, r, api.Conf.Global.WebDir+r.URL.Path)
    } else {
        http.ServeFile(w, r, api.Conf.Global.WebDir+"/index.html")
    }
}

func (api *Api) SetAlerts(data Alerts) {
    for _, value := range data.AlertsArray {

        for _, ext := range api.Conf.ExtensionRules {
            for _, mrs := range ext.Matchers {
                matchers := [][]config.Matcher{ mrs }
                if checkMatches(value.Labels, matchers) {
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

        if value.Status != "" {
            if value.Status != "resolved" {
                if value.Labels["severity"] != nil {
                    value.State = value.Labels["severity"].(string)
                }
            } else {
                value.State = value.Status
            }
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
    
            CacheAlerts.Set(group_id, alert)
    
        } else {

            alert_id := getHash(string(strconv.FormatInt(time.Now().UTC().UnixNano(), 16)+group_id))
            
            var alert cache.Alert

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
    
            CacheAlerts.Set(group_id, alert)

        }

    }
}

func (api *Api) ApiAlerts(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    if r.Method == "GET" {

        _, code, err := authentication(api.Conf.Global.DB, r)
        if err != nil {
            w.WriteHeader(code)
            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
            return
        }

        var alerts Alerts

        limit        := api.Conf.Global.AlertsLimit
        state        := make(map[string]int)
        strArgs      := make(map[string]string)
        intArgs      := make(map[string]int64)
        var matchers [][]config.Matcher

        for k, v := range r.URL.Query() {
            switch k {
                case "alert_id","group_id":
                    strArgs[k] = v[0]
                case "state":
                    for _, st := range strings.Split(v[0], "|") {
                        state[st] = 1
                    }
                case "position","repeat_min","repeat_max":
                    i, err := strconv.Atoi(v[0])
                    if err != nil {
                        w.WriteHeader(400)
                        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
                        return 
                    }
                    intArgs[k] = int64(i)
                case "limit":
                    l, err := strconv.Atoi(v[0])
                    if err != nil {
                        w.WriteHeader(400)
                        w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
                        return 
                    }
                    if l < limit {
                        limit = l
                    }
                case "match[]":
                    for _, s := range v {
                        mrs, err := config.ParseMetricSelector(s)
                        if err != nil {
                            w.WriteHeader(400)
                            w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:make(map[string]string, 0)}))
                            return
                        }
                        matchers = append(matchers, mrs)
                    }
                default:
                    w.WriteHeader(400)
                    w.Write(encodeResp(&Resp{Status:"error", Error:fmt.Sprintf("executing query: invalid parameter '%v'", k), Data:make(map[string]string, 0)}))
                    return
            }
        }

        for _, a := range CacheAlerts.Items() {

            if intArgs["position"] != 0 && intArgs["position"] > a.ActiveAt {
                continue
            }
            if intArgs["repeat_min"] != 0 && intArgs["repeat_min"] > int64(a.Repeat) {
                continue
            }
            if intArgs["repeat_max"] != 0 && intArgs["repeat_max"] < int64(a.Repeat) {
                continue
            }
            if intArgs["position"] != 0 && intArgs["position"] > a.ActiveAt {
                continue
            }
            if strArgs["alert_id"] != "" && strArgs["alert_id"] != a.AlertId {
                continue
            }
            if strArgs["group_id"] != "" && strArgs["group_id"] != a.GroupId {
                continue
            }
            if len(state) != 0 && state[a.State] == 0 {
                continue
            }
            if len(matchers) != 0 && !checkMatches(a.Labels, matchers) {
                continue
            }

            var alert Alert

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
            
            if len(alerts.AlertsArray) >= limit {
                continue
            }
        }

        if len(alerts.AlertsArray) == 0 {
            alerts.AlertsArray = make([]Alert, 0)
        } else {
            if limit == api.Conf.Global.AlertsLimit {
                var warnings []string
                warnings = append(warnings, fmt.Sprintf("display limit exceeded - %d", limit))
                w.Write(encodeResp(&Resp{Status:"success", Warnings:warnings, Data:alerts}))
                return
            }
        }
        
        w.Write(encodeResp(&Resp{Status:"success", Data:alerts}))
        return
    }

    if r.Method == "DELETE" {

        _, code, err := authentication(api.Conf.Global.DB, r)
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

        var alerts Alerts

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

    user, found := CacheUsers.Get(username)
    if found {
        if getHash(password) == user.Password {
            w.WriteHeader(200)
            w.Write(encodeResp(&Resp{Status:"success", Data:user}))
            return
        }
    }

    if api.Conf.Global.Auth.Ldap.Enabled {

        if api.Conf.Global.Auth.Ldap.BindUser == "" && api.Conf.Global.Auth.Ldap.BindPass == "" {
            api.Conf.Global.Auth.Ldap.BindUser = username
            api.Conf.Global.Auth.Ldap.BindPass = password
        }

        var attributes []string
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

        var user cache.User
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
