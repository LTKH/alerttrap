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
    "io/ioutil"
	"encoding/json"
	"github.com/ltkh/alertstrap/internal/db"
	"github.com/ltkh/alertstrap/internal/cache"
	"github.com/ltkh/alertstrap/internal/config"
	//"github.com/prometheus/prometheus/pkg/labels"

)

var (
	CacheAlerts *cache.Alerts = cache.NewCacheAlerts()
	//CacheUsers *cache.Users = cache.NewCacheUsers()
	//re_alert = regexp.MustCompile(`^(\w*)(?:{(.*)})?$`)
	//re_label = regexp.MustCompile(`,?([\w]+)(=|!=|=~|!~)"([^"]*)"`)
	re_labels = regexp.MustCompile(`(?:([\w]+)([=!~]{1,2})"([^"]*)")`)
)

type Api struct {
	Alerts       config.Alerts
}

type Resp struct {
	Status       string                  `json:"status"`
	Error        string                  `json:"error,omitempty"`
	Warnings     []string                `json:"warnings,omitempty"`
	Data         interface{}             `json:"data"`
}

type Alerts struct {
	Stamp        int64                   `json:"stamp"`
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

func getHash(text string) (string) {
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
			if alert.Labels[m.Name] == nil {
                alert.Labels[m.Name] = ""
			}
			if !m.matches(alert.Labels[m.Name].(string)) {
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

func New(config *config.Config) (*Api, error) {
	client, err := db.NewClient(&config.DB); 
	defer client.Close()
	if err != nil {
		return nil, err
	}
	log.Print("[info] connected to dbase")
	alerts, err := client.LoadAlerts()
	if err != nil {
		return nil, err
	}
	for _, alert := range alerts {
		CacheAlerts.Set(alert.GroupId, alert)
	}
	log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
	
	return &Api{ Alerts: config.Alerts }, nil
}

func (api *Api) ApiMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(encodeResp(&Resp{Status:"success"}))
}

func (api *Api) ApiAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {

		var alerts Alerts

		//limit setting
		limit := api.Alerts.Limit
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

        //stamp settings
        stamp := int64(0)
		if r.URL.Query()["stamp"] != nil {
			i, err := strconv.Atoi(r.URL.Query()["stamp"][0])
			if err == nil {
				stamp = int64(i) 
			}
		}

		for _, a := range CacheAlerts.Items() {
            if stamp == 0 || a.StampsAt >= stamp {
				if len(matcherSets) == 0 || checkMatch(&a, matcherSets) {

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

					if a.StampsAt > alerts.Stamp {
						alerts.Stamp  = a.StampsAt
					}
				}
			}
			
			if len(alerts.AlertsArray) >= limit {
				var warnings []string
				if limit == api.Alerts.Limit {
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
					ends_at    = time.Now().UTC().Unix() + api.Alerts.Resolve
				} 
			
				group_id := getHash(string(labels))
			
				alert, found := CacheAlerts.Get(group_id)
				if found {

					if !(alert.Status == "resolved" && value.Status == "resolved") {
						alert.Status         = value.Status
						alert.StartsAt       = starts_at
						alert.StampsAt       = time.Now().UTC().Unix()
						alert.Annotations    = value.Annotations
						alert.GeneratorURL   = value.GeneratorURL
						alert.Duplicate      = alert.Duplicate + 1
					}

					alert.EndsAt         = ends_at
			
					CacheAlerts.Set(group_id, alert)
			
				} else {

					alert_id := getHash(string(strconv.FormatInt(time.Now().UTC().UnixNano(), 16)+group_id))
					
					var alert cache.Alert

					alert.AlertId        = alert_id
					alert.GroupId        = group_id
					alert.Status         = value.Status
					alert.StartsAt       = starts_at
					alert.EndsAt         = ends_at
					alert.StampsAt       = time.Now().UTC().Unix()
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

	_, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[error] %v - %s", err, r.URL.Path)
		w.WriteHeader(400)
		w.Write([]byte(fmt.Sprintf("{\"status\":\"error\",\"error\":\"%s\",\"data\":{}}", err.Error())))
		return
	}

	w.WriteHeader(204)
	return

}
