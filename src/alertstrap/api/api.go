package api

import (
  "net/http"
  "log"
  //"crypto/sha1"
  //"encoding/base64"
  "strings"
  "regexp"
  "time"
  "strconv"
  //"io/ioutil"
  "encoding/json"
  "alertstrap/cache"
  "alertstrap/db"
  "alertstrap/config"
)

type Api struct {

}

var (
  Alerts *cache.Cache = cache.New(30 * time.Minute, 5 * time.Minute)
  Hosts  *cache.Cache = cache.New(30 * time.Minute, 5 * time.Minute)
)

/*
func getHash(bv []byte) (string) {
  hasher := sha1.New()
  hasher.Write(bv)
  return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func addAlert(data map[string]interface{}) error {

  now := time.Now().Format("2006-01-02T03:04:05")
  //now.Format(time.RFC3339)
  //now.Format("YYYY.MM.DD-hh.mm.ss")

  if data["host"]     == nil { data["host"]     = "localhost" }
  if data["severity"] == nil { data["severity"] = "unknown" }
  if data["sv_level"] == nil { data["sv_level"] = 0 }
  if data["param"]    == nil { data["param"]    = "" }
  if data["appl_id"]  == nil { data["appl_id"]  = "" }
  if data["instance"] == nil { data["instance"] = "" }
  if data["object"]   == nil { data["object"]   = "" }
  if data["text"]     == nil { data["text"]     = "" }

  data["severity"] = strings.ToLower(data["severity"].(string))
  data["ts_unix"]  = int64(time.Now().Unix())
  data["mgrp_id"]  = getHash([]byte(data["host"].(string)+data["param"].(string)+data["appl_id"].(string)+data["instance"].(string)+data["object"].(string)))
  data["mess_id"]  = getHash([]byte(string(data["ts_unix"].(int64))+data["mgrp_id"].(string)+data["severity"].(string)))

  alert, found := Alerts.Get(data["mgrp_id"].(string))
	if found {
    alert["duplicate"] = alert["duplicate"].(int) + 1
    alert["text"] = data["text"]
    alert["severity"] = data["severity"]
    alert["sv_level"] = data["sv_level"]
    alert["ts_max"] = now
    alert["ts_unix"] = data["ts_unix"]
    if err := db.UpdAlert(alert, alert["mess_id"].(string)); err != nil {
      return err
    }
    Alerts.Set(data["mgrp_id"].(string), alert, 30 * time.Minute)
	} else {
    data["duplicate"] = 1
    data["ts_min"] = now
    data["ts_max"] = now
    if err := db.AddAlert(data); err != nil {
      return err
    }
    Alerts.Set(data["mgrp_id"].(string), data, 30 * time.Minute)
  }
  return nil
}

func LoadHosts() error {
  hosts, err := db.LoadHosts()
  if err != nil {
    return err
  }
  for _, host := range hosts {
    Hosts.Set(host["host"].(string), host, 30 * time.Minute)
  }
  return nil
}
*/

func LoadAlerts(conf config.Config) error {
  alerts, err := db.LoadAlerts(conf)
  if err != nil {
    return err
  }
  for _, alert := range alerts {
    Alerts.Set(alert["mgrp_id"].(string), alert, 30 * time.Minute)
  }
  log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
  return nil
}

func checkMatch(alrt cache.Item, prms map[string]*regexp.Regexp, unix int64) bool {
  if alrt.Value["ts_unix"].(int64) < unix {
    return false
  }
  for key, prm := range prms {
    val := ""
    switch alrt.Value[key].(type) {
    	case int:
    		val = strconv.Itoa(alrt.Value[key].(int))
    	case string:
    		val = alrt.Value[key].(string)
    	default:
    		return false
  	}
    if !prm.Match([]byte(val)) {
      return false
    }
  }
  return true
}

func encodeJson(data interface{}) []byte {
  jsn, err := json.Marshal(data)
  if err != nil {
    return []byte("[]")
  }
  return jsn
}

//HTTP Server

func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {

  if r.URL.Path == "/get/alerts" {

    unix := int64(0)
    prms := make(map[string]*regexp.Regexp)
    re := regexp.MustCompile(",")
    for key, value := range r.URL.Query() {
      if key == "timestamp" {
        i, err := strconv.Atoi(value[0])
        if err == nil {
          unix = int64(i)
        }
      } else {
        st := re.ReplaceAllString("("+strings.Join(value, "|")+")", `|`)
        prms[key] = regexp.MustCompile(string(st))
      }
    }

    var alts []map[string]interface{}
    for _, alrt := range Alerts.Items() {
      host, ok := Hosts.Get(alrt.Value["host"].(string))
      if ok {
        for key, value := range host {
          alrt.Value[key] = value
        }
      }
      if checkMatch(alrt, prms, unix) {
        alts = append(alts, alrt.Value)
      }
      if len(alts) >= 5000 {
        break;
      }
    }

    if len(alts) > 0 {
      w.Write(encodeJson(alts))
      return
    }

    w.Write([]byte("[]"))
    return
  }


  /*
  if r.URL.Path == "/get/history" {

    history, err := db.GetHistory(r.URL.Query())
    if err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
      w.WriteHeader(500)
      w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
      return
    }

    w.Write(encodeJson(history))
    return
  }

  if r.URL.Path == "/get/hosts" {

    hosts, err := db.LoadHosts()
    if err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
      w.WriteHeader(500)
      w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
      return
    }

    w.Write(encodeJson(hosts))
    return
  }

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Printf("[error] %v - %s", err, r.URL.Path)
    w.WriteHeader(400)
    w.Write([]byte("{\"error\":\"read request body\"}"))
    return
  }

  if r.URL.Path == "/add/alert" {

    var data map[string]interface{}

    if err := json.Unmarshal(body, &data); err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
      w.WriteHeader(400)
      w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
      return
    }

    if err := addAlert(data); err != nil {
  		log.Printf("[error] %v - %s", err, r.URL.Path)
      w.WriteHeader(400)
      w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
      return
  	}

    w.WriteHeader(204)
    return
  }
  */

  w.WriteHeader(404)
  return
}

//"/get/profile"
//"/get/profiles"
//"/set/profile"
//"/get/services"
//"/get/menu"
//"/get/history"
//"/get/information"
//"/set/information"
