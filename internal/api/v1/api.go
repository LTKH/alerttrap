package v1

import (
    "io"
	"net/http"
    "log"
	"fmt"
    "crypto/sha1"
    "encoding/hex"
    "regexp"
    "time"
	"strconv"
	"errors"
    "io/ioutil"
	"encoding/json"
	"github.com/ltkh/alertstrap/internal/db"
	"github.com/ltkh/alertstrap/internal/cache"
	"github.com/ltkh/alertstrap/internal/config"
	"github.com/ltkh/alertstrap/internal/ldap"
)

var (
	CacheAlerts *cache.Alerts = cache.NewCacheAlerts()
	CacheUsers *cache.Users = cache.NewCacheUsers()
	re_labels = regexp.MustCompile(`(?:([\w]+)([=!~]{1,2})"([^"]*)")`)
)

type Api struct {
	Conf         *config.Config
}

type Resp struct {
	Status       string                  `json:"status"`
	Error        string                  `json:"error,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
	Data         interface{}             `json:"data"`
}

type Alerts struct {
	Position     int64                   `json:"position"`
  	AlertsArray  []Alert                 `json:"alerts"`
}

type Alert struct {
  	AlertId      string                  `json:"alertId"`
  	GroupId      string                  `json:"groupId"`
	Status       string                  `json:"status"`
  	StartsAt     time.Time               `json:"startsAt"`
  	EndsAt       time.Time               `json:"endsAt"`
  	Duplicate    int                     `json:"duplicate"`
  	Labels       map[string]interface{}  `json:"labels"`
  	Annotations  map[string]interface{}  `json:"annotations"`
  	GeneratorURL string                  `json:"generatorURL"`
}

// Matcher models the matching of a label.
type Matcher struct {
	Type  string
	Name  string
	Value string
	re *regexp.Regexp
}

// NewMatcher returns a matcher object.
func newMatcher(t, n, v string) (*Matcher, error) {
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
		m.re = re
	}
	return m, nil
}

// Matches returns whether the matcher matches the given string value.
func (m *Matcher) matches(s string) bool {
	switch m.Type {
	case "=":
		return s == m.Value
	case "!=":
		return s != m.Value
	case "=~":
		return m.re.MatchString(s)
	case "!~":
		return !m.re.MatchString(s)
	}
	return false
}

func encodeResp(resp *Resp) []byte {
    jsn, err := json.Marshal(resp)
	if err != nil {
		return encodeResp(&Resp{Status:"error", Error:err.Error()})
	}
	return jsn
}

func getHash(text string) string {
	h := sha1.New()
	io.WriteString(h, text)
	return hex.EncodeToString(h.Sum(nil))
}

func parseMetricSelector(input string) (m []*Matcher, err error) {
	var matchers []*Matcher

	lbls := re_labels.FindAllStringSubmatch(input, -1)
	for _, l := range lbls {

		matcher, err := newMatcher(l[2], l[1], l[3])
		if err != nil {
			return nil, err
		}

		matchers = append(matchers, matcher)
	}

	return matchers, nil
}

func checkMatch(alert *cache.Alert, matchers [][]*Matcher) bool {
	for _, mtch := range matchers {
        match := true

		for _, m := range mtch {
			val := alert.Labels[m.Name]
			if val == nil {
                val = ""
			}
			if !m.matches(val.(string)) {
				match = false
			    break
			}
		}

		if match {
			return true
		}
	}

	return false
}

func authentication(cfg config.DB, r *http.Request) (int, error) {
	var login, token string

	login, token, ok := r.BasicAuth()
    if !ok {
		lg, err := r.Cookie("login")
		if err != nil {
			return 401, errors.New("Unauthorized")
		}
		login = lg.Value
		tk, err := r.Cookie("token")
		if err != nil {
			return 401, errors.New("Unauthorized")
		}
		token = tk.Value
	}

	if login == "" || token == "" {
		return 401, errors.New("Unauthorized")
	}

	user, ok := CacheUsers.Get(login)
	if !ok { 
		cln, err := db.NewClient(&cfg)
		if err != nil {
			return 500, err
		}
		usr, err := cln.LoadUser(login)
		if err != nil {
			return 403, err
		}
		CacheUsers.Set(login, usr)
	} else {
		if user.Token != token {
			return 403, errors.New("Forbidden")
		}
	}

	return 204, nil
}

func New(conf *config.Config) (*Api, error) {
	//connection to data base
	client, err := db.NewClient(&conf.DB)
	if err != nil {
		return nil, err
	}
	log.Print("[info] connected to dbase")
	//loading alerts
	alerts, err := client.LoadAlerts()
	if err != nil {
		return nil, err
	}
	for _, alert := range alerts {
		CacheAlerts.Set(alert.GroupId, alert)
	}
	log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
	//loading users
	users, err := client.LoadUsers()
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		CacheUsers.Set(user.Login, user)
	}
	log.Printf("[info] loaded users from dbase (%d)", len(users))
	
	return &Api{ Conf: conf }, nil
}

func (api *Api) ApiAuth(w http.ResponseWriter, r *http.Request) {
    code, err := authentication(api.Conf.DB, r)
	if err != nil {
		w.WriteHeader(code)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}
	w.WriteHeader(code)
	w.Write(encodeResp(&Resp{Status:"success"}))
}

func (api *Api) ApiMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	/*
	jsn, err := json.Marshal(api.Menu)
	if err != nil {
		log.Printf("[error] %v - %s", err, r.URL.Path)
		w.WriteHeader(400)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}
	*/

	//log.Printf("[error] %q", api.Menu)
	
	//var nodes Nodes;
	//for _, m := range api.Menu {
	//	for _, v := range m.Section {
	//		log.Printf("%v - %v", m.Name, v.Name)
	//	}
	//}
	w.Write(encodeResp(&Resp{Status:"success", Data:api.Conf.Menu}))
}

func (api *Api) ApiAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {

		var alerts Alerts

		code, err := authentication(api.Conf.DB, r)
		if err != nil {
			w.WriteHeader(code)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:alerts}))
			return
		}

		//limit setting
		limit := api.Conf.Alerts.Limit
		if r.URL.Query()["limit"] !=nil {
			l, err := strconv.Atoi(r.URL.Query()["limit"][0])
			if err == nil && l < limit {
                limit = l
			}
		}

		//match setting
		var matcherSets [][]*Matcher
		for _, s := range r.URL.Query()["match[]"] {
			matchers, err := parseMetricSelector(s)
			if err != nil {
				log.Printf("[error] %v - %s", err, r.URL.Path)
				w.WriteHeader(400)
				w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:alerts}))
				return
			}
			matcherSets = append(matcherSets, matchers)
		}

        //position settings
        position := int64(0)
		if r.URL.Query()["position"] != nil {
			i, err := strconv.Atoi(r.URL.Query()["position"][0])
			if err == nil {
				position = int64(i) 
			}
		}

		//status settings
		var re_status *regexp.Regexp
		if r.URL.Query()["status"] != nil {
			re, err := regexp.Compile("^(?:" + r.URL.Query()["status"][0] + ")$")
			if err != nil {
				log.Printf("[error] %v - %s", err, r.URL.Path)
				w.WriteHeader(400)
				w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:alerts}))
				return 
			}
			re_status = re
		}

		for _, a := range CacheAlerts.Items() {

            if position != 0 && a.ActiveAt < position {
                continue
			}
			if re_status != nil && !re_status.MatchString(a.Status) {
                continue
			}
			if len(matcherSets) != 0 && !checkMatch(&a, matcherSets) {
                continue
			}

			var alert Alert

			alert.AlertId      = a.AlertId
			alert.GroupId      = a.GroupId
			alert.Status       = a.Status
			alert.StartsAt     = time.Unix(a.StartsAt, 0)
			alert.EndsAt       = time.Unix(a.EndsAt, 0)
			alert.Duplicate    = a.Duplicate
			alert.Labels       = a.Labels
			alert.Annotations  = a.Annotations
			alert.GeneratorURL = a.GeneratorURL

			alerts.AlertsArray = append(alerts.AlertsArray, alert)

			if a.ActiveAt > alerts.Position {
				alerts.Position  = a.ActiveAt
			}
			
			if len(alerts.AlertsArray) >= limit {
				var warnings []string
				if limit == api.Conf.Alerts.Limit {
					warnings = append(warnings, fmt.Sprintf("display limit exceeded - %d", limit))
				}
				w.Write(encodeResp(&Resp{Status:"success", Warnings:warnings, Data:alerts}))
				return
			}
		}
		
		w.Write(encodeResp(&Resp{Status:"success", Data:alerts}))
		return
	}

    if r.Method == "POST" {

		var alerts []Alert

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("[error] %v - %s", err, r.URL.Path)
			w.WriteHeader(400)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
			return
		}

		if err := json.Unmarshal(body, &alerts); err != nil {
			log.Printf("[error] %v - %s", err, r.URL.Path)
			w.WriteHeader(400)
			w.Write(encodeResp(&Resp{Status:"error", Error:err.Error(), Data:alerts}))
			return
		}

		go func(data []Alert){

			for _, value := range data {

				labels, err := json.Marshal(value.Labels)
				if err != nil {
					log.Printf("[error] read alert %v", err)
					return
				}

				if value.Status == "" {
                    value.Status = "firing"
				}
			
				starts_at := value.StartsAt.UTC().Unix()
				ends_at   := value.EndsAt.UTC().Unix()
				if starts_at < 0 {
					starts_at  = time.Now().UTC().Unix()
				} 
				if ends_at < 0 {
					ends_at    = time.Now().UTC().Unix() + api.Conf.Alerts.Resolve
				} 
			
				group_id := getHash(string(labels))
			
				alert, found := CacheAlerts.Get(group_id)
				if found {

					if !(alert.Status == "resolved" && value.Status == "resolved") {
						alert.Status         = value.Status
						alert.ActiveAt       = time.Now().UTC().Unix()
						alert.StartsAt       = starts_at
						alert.Annotations    = value.Annotations
						alert.GeneratorURL   = value.GeneratorURL
						alert.Duplicate      = alert.Duplicate + 1
					}

					alert.EndsAt = ends_at
			
					CacheAlerts.Set(group_id, alert)
			
				} else {

					alert_id := getHash(string(strconv.FormatInt(time.Now().UTC().UnixNano(), 16)+group_id))
					
					var alert cache.Alert

					alert.AlertId        = alert_id
					alert.GroupId        = group_id
					alert.Status         = value.Status
					alert.ActiveAt       = time.Now().UTC().Unix()
					alert.StartsAt       = starts_at
					alert.EndsAt         = ends_at
					alert.Labels         = value.Labels
					alert.Annotations    = value.Annotations
					alert.GeneratorURL   = value.GeneratorURL
					alert.Duplicate      = 1
			
					CacheAlerts.Set(group_id, alert)

				}

			}

		}(alerts)

		w.WriteHeader(204)
		return
	}

	w.WriteHeader(405)
	w.Write(encodeResp(&Resp{Status:"error", Error:"Method Not Allowed"}))
}

func (api *Api) ApiLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(400)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}

	r.ParseForm()
	username := r.Form.Get("login")
	password := r.Form.Get("password")

	if username == "" || password == "" {
		w.WriteHeader(403)
		w.Write(encodeResp(&Resp{Status:"error", Error:"Login or password is empty"}))
		return
	}

	if api.Conf.Ldap.Bind_user == "" && api.Conf.Ldap.Bind_pass == "" {
		api.Conf.Ldap.Bind_user = username
		api.Conf.Ldap.Bind_pass = password
	}

	var attributes []string
	for _, val := range api.Conf.Ldap.Attributes {
		attributes = append(attributes, val)
	}

	clnt := &ldap.LDAPClient{
		Base:         api.Conf.Ldap.Base,
		Host:         api.Conf.Ldap.Host,
		Port:         api.Conf.Ldap.Port,
		UseSSL:       api.Conf.Ldap.Use_ssl,
		BindDN:       fmt.Sprintf(api.Conf.Ldap.Bind_dn, api.Conf.Ldap.Bind_user),
		BindPassword: api.Conf.Ldap.Bind_pass,
		UserFilter:   api.Conf.Ldap.User_filter,
		GroupFilter:  api.Conf.Ldap.Group_filter,
		Attributes:   attributes,
	}
	defer clnt.Close()

	ok, usr, err := clnt.Authenticate("username", "password")
	if err != nil {
		log.Printf("[error] authenticating user %s: %+v", username, err)
		w.WriteHeader(403)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}
	if !ok {
		log.Printf("[error] authenticating user %s: %+v", username, err)
		w.WriteHeader(403)
		w.Write(encodeResp(&Resp{Status:"error", Error:err.Error()}))
		return
	}

	var user cache.User
	user.Login = username
	user.Name = usr["username"]
	user.Password = getHash(password)
	user.Token = getHash(string(time.Now().UTC().Unix()))
	
	CacheUsers.Set(username, user)

	cln, err := db.NewClient(&api.Conf.DB)
	if err == nil {
		cln.SaveUser(user)
	} else {
		log.Printf("[error] %v", err)
	}

	w.Write(encodeResp(&Resp{Status:"success", Data:user}))
	return

}
