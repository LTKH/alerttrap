package api

import (
  "io"
  "net/http"
  "net/url"
  "log"
  "crypto/sha1"
  "encoding/hex"
  //"encoding/base64"
  "regexp"
  "time"
  "strconv"
  //"strings"
  "io/ioutil"
  "encoding/json"
  "alertstrap/cache"
  "alertstrap/db"
)

var (
  CacheAlerts *cache.Cache = cache.New()
  //ChanAlrets = make(chan *cache.Alert)
)

type Alerts struct {
  AlertsArray []Alert `json:"alerts"`
}

type Alert struct {
  AlertId      string                  `json:"alertId"`
  GroupId      string                  `json:"groupId"`
  Status       string                  `json:"status"`
  StartsAt     time.Time               `json:"startsAt"`
  EndsAt       time.Time               `json:"endsAt"`
  Epoch        int64                   `json:"epoch"`
  Duplicate    int                     `json:"duplicate"`
  Labels       map[string]interface{}  `json:"labels"`
  Annotations  map[string]interface{}  `json:"annotations"`
  GeneratorURL string                  `json:"generatorURL"`
}

func getHash(text string) (string) {

  h := sha1.New()
  io.WriteString(h, text)
  return hex.EncodeToString(h.Sum(nil))
}

func checkMatch(labels map[string]interface{}, values url.Values) bool {
  
  for key, array := range values {
    if key != "epoch" && labels[key] != nil {
      match := false
      for _, val := range array {
        mtch, err := regexp.MatchString(val, labels[key].(string))
        if err != nil {
          log.Printf("[error] %v", err)
        }
        if mtch {
          match = true
          break
        }
      }
      return match
    }
  }

  return true
}

/*

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

func LoadAlerts() error {
  alerts, err := db.LoadAlerts()
  if err != nil {
    return err
  }
  for _, alert := range alerts {
    CacheAlerts.Set(alert.GroupId, alert, alert.EndsAt + 1800)
  }
  log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
  return nil
}

func GetAlerts(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")

  epoch := int64(0)
  if r.URL.Query()["epoch"] != nil {
    i, err := strconv.Atoi(r.URL.Query()["epoch"][0])
    if err == nil {
      epoch = int64(i) 
    }
  }

  var alerts Alerts
  for _, a := range CacheAlerts.Items() {
    
    if a.Value.EndsAt >= epoch && checkMatch(a.Value.Labels, r.URL.Query()) {

      var alert Alert

      alert.AlertId      = a.Value.AlertId
      alert.GroupId      = a.Value.GroupId
      alert.Status       = a.Value.Status
      alert.StartsAt     = time.Unix(a.Value.StartsAt, 0)
      alert.EndsAt       = time.Unix(a.Value.EndsAt, 0)
      alert.Epoch        = a.Value.EndsAt
      alert.Duplicate    = a.Value.Duplicate
      alert.Labels       = a.Value.Labels
      alert.Annotations  = a.Value.Annotations
      alert.GeneratorURL = a.Value.GeneratorURL

      alerts.AlertsArray = append(alerts.AlertsArray, alert)

    }
    
    if len(alerts.AlertsArray) >= 5000 {
      break;
    }
  }

  if len(alerts.AlertsArray) == 0 {
    w.Write([]byte("{\"alerts\":[]}"))
    return
  }

  jsn, err := json.Marshal(alerts)
  if err != nil {
    w.Write([]byte("{\"alerts\":[]}"))
    return 
  }
  w.Write(jsn)
  return

}

func AddAlerts(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")

  body, err := ioutil.ReadAll(r.Body)
  if err != nil {
    log.Printf("[error] %v - %s", err, r.URL.Path)
    w.WriteHeader(400)
    w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
    return
  }

  var data Alerts

  if err := json.Unmarshal(body, &data); err != nil {
    log.Printf("[error] %v - %s", err, r.URL.Path)
    w.WriteHeader(400)
    w.Write([]byte("{\"error\":\""+err.Error()+"\"}"))
    return
  }

  go func(data *Alerts){

    for _, value := range data.AlertsArray {

      labels, err := json.Marshal(value.Labels)
      if err != nil {
        log.Printf("[error] read alert %v", err)
        return
      }
  
      starts_at := value.StartsAt.UTC()
      ends_at   := value.EndsAt.UTC()
      if starts_at.Unix() < 0 {
        starts_at  = time.Now().UTC()
      } 
      if ends_at.Unix() < 0 {
        ends_at    = time.Now().UTC()
      } 
  
      group_id  := getHash(string(labels));
      alert_id  := getHash(string(strconv.FormatInt(starts_at.UnixNano(), 16)+group_id))
  
      /*
      select {
        case ChanAlrets <- &cache.Alert{
          AlertId:       alert_id,
          GroupId:       group_id,
          Status:        value.Status,
          StartsAt:      starts_at.Unix(),
          EndsAt:        ends_at.Unix(),
          Labels:        value.Labels,
          Annotations:   value.Annotations,
          GeneratorURL:  value.GeneratorURL,
        }:
        default:
          log.Print("[error] channel to alerts is not ready")
      }
      */
  
      alert, found := CacheAlerts.Get(group_id)
      if found {
        alert.Status         = value.Status
        alert.Annotations    = value.Annotations
        alert.GeneratorURL   = value.GeneratorURL
        alert.Duplicate      = alert.Duplicate + 1
        alert.EndsAt         = ends_at.Unix()
  
        if err := db.UpdAlert(alert); err != nil {
          log.Printf("[error] update alert %v", err)
          return
        }
        CacheAlerts.Set(group_id, alert, alert.EndsAt + 1800)
  
      } else {
        var alert cache.Alert
        alert.AlertId        = alert_id
        alert.GroupId        = group_id
        alert.Status         = value.Status
        alert.StartsAt       = starts_at.Unix()
        alert.EndsAt         = ends_at.Unix()
        alert.Labels         = value.Labels
        alert.Annotations    = value.Annotations
        alert.GeneratorURL   = value.GeneratorURL
        alert.Duplicate      = 1
  
        if err := db.AddAlert(alert); err != nil {
          log.Printf("[error] add alert %v", err)
          return
        }
        CacheAlerts.Set(group_id, alert, alert.EndsAt + 1800)
      }
    }

  }(&data)

  w.WriteHeader(204)
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

  
  */
//}

//"/get/profile"
//"/get/profiles"
//"/set/profile"
//"/get/services"
//"/get/menu"
//"/get/history"
//"/get/information"
//"/set/information"
