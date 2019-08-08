package api

import (
  "net/http"
  "log"
  "crypto/sha1"
  "encoding/base64"
  "strings"
  //"reflect"
  //"regexp"
  "time"
  "io/ioutil"
  "encoding/json"
  //"alertstrap/db"
  //"alertstrap/config"
  "github.com/patrickmn/go-cache"
)

var (
  Alerts *cache.Cache   = cache.New(30*time.Minute, 5*time.Minute)
  //Config *config.Config = config.LoadConfigFile("conf/config.toml")
)

func getHash(bv []byte) (string) {
  hasher := sha1.New()
  hasher.Write(bv)
  return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

/*
func LoadAlerts() bool {
  var alerts []map[string]interface{}
  for _, alrt := range db.LoadAlerts() {
    Alerts = append(Alerts, alrt)
  }
  log.Print("[info] alerts loaded from database")
  return true
}
*/

func addAlert(dat map[string]interface{}) bool {
  var datetime = time.Now()
  datetime.Format(time.RFC3339)

  if dat["host"]     == nil { dat["host"]     = "localhost" }
  if dat["severity"] == nil { dat["severity"] = "unknown" }
  if dat["sv_level"] == nil { dat["sv_level"] = 0 }
  if dat["param"]    == nil { dat["param"]    = "" }
  if dat["appl_id"]  == nil { dat["appl_id"]  = "" }
  if dat["instance"] == nil { dat["instance"] = "" }
  if dat["object"]   == nil { dat["object"]   = "" }
  if dat["text"]     == nil { dat["text"]     = "" }

  dat["sv_level"] = strings.ToLower(dat["severity"].(string))
  dat["ts_unix"]  = int32(time.Now().Unix())
  dat["mgrp_id"]  = getHash([]byte(dat["host"].(string)+dat["param"].(string)+dat["appl_id"].(string)+dat["instance"].(string)+dat["object"].(string)))
  dat["mess_id"]  = getHash([]byte(dat["ts_unix"].(string)+dat["mgrp_id"].(string)+dat["severity"].(string)))

  alert, found := Alerts.Get(dat["mgrp_id"].(string))
	if found {
    //alert[dat["mgrp_id"]].duplicate += 1
    //alert[dat["mgrp_id"]].text = dat.text
    //alert[dat["mgrp_id"]].severity = dat.severity
    //alert[dat["mgrp_id"]].sv_level = dat.sv_level
    //alert[dat["mgrp_id"]].ts_max = datetime
    //alert[dat["mgrp_id"]].ts_unix = dat.ts_unix
    //Alerts.Set(dat["mgrp_id"], alert, cache.DefaultExpiration)
    //go db.UpdAlert(Alerts[dat["mgrp_id"]])
	} else {
    var i interface{}
    dat["ts_min"] = datetime
    dat["ts_max"] = datetime
    i = dat
    Alerts.Set(dat["mgrp_id"].(string), i, cache.DefaultExpiration)
    //go db.AddAlert(Alerts[dat["mgrp_id"]])
  }
  return true
}


/*
func getFieldString(a map[string]interface{}, field string) string {
  r := reflect.ValueOf(a)
  f := reflect.Indirect(r).FieldByName(field)

  return f.String()
}

func checkMatch(alrt map[string]interface{}, prms map[string]*regexp.Regexp) bool {
  for key, prm := range prms {
    field := getFieldString(alrt, strings.Title(key))
    if !prm.Match([]byte(field)) {
      return false
    }
  }
  return true
}



func GetAlerts() {
  //for _, alrt := range alerts.Items() {
    log.Printf("[info] %v", alerts.Items())
  //}
}



*/

func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {

  if r.URL.Path == "/get/alerts" {
/*
    prms := make(map[string]*regexp.Regexp)
    re := regexp.MustCompile(",")
    for key, values := range r.URL.Query() {
      st := re.ReplaceAllString("("+strings.Join(values, "|")+")", `|`)
      prms[key] = regexp.MustCompile(string(st))
    }
*/

    var alts []cache.Item
    for _, alrt := range Alerts.Items() {
      //if checkMatch(alrt, prms) { alts = append(alts, alrt) }
      alts = append(alts, alrt)
      //log.Printf("[error] %v", alrt)
    }

    jsn, err := json.Marshal(alts)
    if err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
      return
    }

    w.Write([]byte(jsn))

    return
  }

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Printf("[error] %v - %s", err, r.URL.Path)
    w.WriteHeader(400)
    return
  }

  if r.URL.Path == "/add/alert" {
     var dat map[string]interface{}

    if err := json.Unmarshal(body, &dat); err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
    }

    addAlert(dat)

    w.WriteHeader(204)
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

}
