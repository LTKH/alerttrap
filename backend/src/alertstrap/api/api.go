package api

import (
  "net/http"
  "log"
  "crypto/sha1"
  "encoding/base64"
  "strings"
  "time"
  "io/ioutil"
  "encoding/json"
  "alertstrap/db"
  "alertstrap/config"
)

var (
  Alerts = make(map[string](*db.Alert))
)

type Api struct {
  Cfg config.Config
}

func LoadAlerts() {
  //for _, value := range db.LoadAlerts() {
  //  Alerts[value.Mess_id] = value
  //}
  //log.Print("[info] alerts loaded from database")
  return
}

func GetAlerts() (map[string](*db.Alert)) {
  return Alerts
}

func DelAlert(id string) {
  delete(Alerts, id)
  return
}

func GetHash(bv []byte) (string) {
  hasher := sha1.New()
  hasher.Write(bv)
  return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func GetSvLevel(lv string) (int) {
  return 5
}

func AddAlert(dat *db.Alert) {
  var datetime = time.Now()
  datetime.Format(time.RFC3339)
  dat.Ts_unix  = int32(time.Now().Unix())
  dat.Severity = strings.ToLower(dat.Severity)
  dat.Sv_level = GetSvLevel(dat.Severity)
  dat.Mgrp_id  = GetHash([]byte(dat.Host+dat.Param+dat.Instance+dat.Object))
  dat.Mess_id  = GetHash([]byte(string(dat.Ts_unix)+dat.Mgrp_id+dat.Severity))

  if _, ok := Alerts[dat.Mgrp_id]; ok {
    Alerts[dat.Mgrp_id].Duplicate += 1
    Alerts[dat.Mgrp_id].Text = dat.Text
    Alerts[dat.Mgrp_id].Severity = dat.Severity
    Alerts[dat.Mgrp_id].Sv_level = dat.Sv_level
    Alerts[dat.Mgrp_id].Zone = dat.Zone
    Alerts[dat.Mgrp_id].Stand = dat.Stand
    Alerts[dat.Mgrp_id].Ts_max = datetime
    Alerts[dat.Mgrp_id].Ts_unix = dat.Ts_unix
    go db.UpdAlert(Alerts[dat.Mgrp_id])
  } else {
    Alerts[dat.Mgrp_id] = dat
    Alerts[dat.Mgrp_id].Ts_min = datetime
    Alerts[dat.Mgrp_id].Ts_max = datetime
    Alerts[dat.Mgrp_id].Duplicate = 1
    go db.AddAlert(Alerts[dat.Mgrp_id])
  }
  return
}

func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {

  if r.URL.Path == "/get/alerts" {

    var alts []*db.Alert
    for _, value := range GetAlerts() {
      alts = append(alts, value)
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
     var dat db.Alert

    if err := json.Unmarshal(body, &dat); err != nil {
      log.Printf("[error] %v - %s", err, r.URL.Path)
    }
    if dat.Host == "" {
      w.Write([]byte("{\"error\": \"field host\"}"))
      return
    }
    if dat.Severity == "" {
      w.Write([]byte("{\"error\": \"field severity\"}"))
      return
    }
    if dat.Text == "" {
      w.Write([]byte("{\"error\": \"field text\"}"))
      return
    }

    AddAlert(&dat)

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
