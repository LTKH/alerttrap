package main

import (
  "net/http"
  "crypto/tls"
  "time"
  "log"
  "os"
  "os/signal"
  "syscall"
  "encoding/json"
  "io/ioutil"
  "bytes"
  "runtime"
  "reflect"
  "text/template"
  "flag"
  "alertstrap/db"
  "alertstrap/config"
  "regexp"
  //"errors"
  //"github.com/jmoiron/sqlx"
  //_ "github.com/go-sql-driver/mysql"
)

var (
  cfg  config.Config
)

func newRequest(method string, url string, jsonStr []byte, login string, password string) ([]byte, error) {

  //ignore certificate
  http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

  req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonStr))
  if err != nil {
    return nil, err
  }
  req.SetBasicAuth(login, password)
  req.Header.Set("Content-Type", "application/json")

  client := &http.Client{}
  resp, err := client.Do(req)
  if err != nil {
    return nil, err
  }
  defer resp.Body.Close()

  body, err := ioutil.ReadAll(resp.Body)
  if err != nil {
    return nil, err
  }

  var dat interface{}
  if err := json.Unmarshal(body, &dat); err != nil {
    log.Printf("[error] %v", err)
    return nil, err
  }

  return body, nil

}

func newTemplate(name string, vals interface{}) []byte {
  tmpl, err := template.ParseGlob(cfg.Jiramanager.Templates+"/*.tmpl")
  if err != nil {
    log.Printf("[error] %v", err)
    return []byte("")
  }

  var tpl bytes.Buffer
  if err = tmpl.ExecuteTemplate(&tpl, name, &vals); err != nil {
    log.Printf("[error] %v", err)
    return []byte("")
  }

  re := regexp.MustCompile(`\{[\s\S]*\}`)
  if re.MatchString(tpl.String()) == false {
    return []byte("")
  }

  return tpl.Bytes()
}

func parseJson(jsn []byte) (map[string]interface{}, error) {
  var dat map[string]interface{}
  if err := json.Unmarshal(jsn, &dat); err != nil {
    log.Printf("[error] %v", err)
    return nil, err
  }
  return dat, nil
}

func searchTask(alrt map[string]interface{}) bool {
  if cfg.Jiramanager.Debug { log.Print("[debug] serching old task") }

  alrt["task_key"] = db.GetTaskKey(alrt["mgrp_id"].(string))
  if alrt["task_key"] == "" {
    if cfg.Jiramanager.Debug { log.Print("[debug] task_key is empty") }
    return true
  }

  def := newTemplate("search", alrt)
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(def)) }

  sch, err := newRequest("POST", cfg.Jiramanager.Jira_api+"/search", def, cfg.Jiramanager.Login, cfg.Jiramanager.Passwd)
  if err != nil {
    if cfg.Jiramanager.Debug { log.Printf("[debug] %v", err) }
    return false
  }
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(sch)) }

  js, err := parseJson(sch)
  if err != nil {
    log.Printf("[error] %v", err)
    return false
  }

  if js["issues"] != nil {
    arr := js["issues"].([]interface{})
    if len(arr) > 0 { return false }
  }

  return true
}

func createTask(alrt map[string]interface{}) bool {
  if cfg.Jiramanager.Debug { log.Print("[debug] creating new task") }

  def := newTemplate("default", alrt)
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(def)) }

  _, err := parseJson(def)
  if err != nil {
    log.Printf("[error] %v", err)
    return false
  }

  crt, err := newRequest("POST", cfg.Jiramanager.Jira_api+"/issue", def, cfg.Jiramanager.Login, cfg.Jiramanager.Passwd)
  if err != nil {
    log.Printf("[error] %v", err)
    return false
  }
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(crt)) }

  js, err := parseJson(crt)
  if err != nil {
    log.Printf("[error] %v", crt)
    return false
  }
  if js["errorMessages"] != nil {
    log.Printf("[error] %v", string(crt))
    return false
  }

  if js["key"] != nil {
    db.UpdateTask(alrt["mgrp_id"].(string), js["key"].(string))
  }

  return true
}

func main() {

  //limits the number of operating system threads
  runtime.GOMAXPROCS(runtime.NumCPU())

  //command-line flag parsing
  cfFile := flag.String("config", "", "config file")
  flag.Parse()

  //loading configuration file
  cfg, err := config.LoadConfigFile(*cfFile)
  if err != nil {
    log.Fatalf("[error] %v", err)
  }

  //program completion signal processing
  c := make(chan os.Signal, 2)
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)
  go func() {
    <- c
    log.Print("[info] jiramanager stopped")
    os.Exit(0)
  }()

  //daemon mode
  for {

    //database connection
    db.Conn = db.ConnectDb(cfg)
    defer db.Conn.Close()

    //
    if cfg.Jiramanager.Debug { log.Print("[debug] running get_alerts function") }
    body, err := newRequest("GET", cfg.Alertstrap.Get_alerts, []byte(""), "", "")
    if err != nil {
      log.Printf("[error] %v", err)
    } else {

      //
      if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(body)) }
      var dat []map[string]interface{}
      if err := json.Unmarshal(body, &dat); err != nil {
          log.Printf("[error] %v", err)
      }

      //
      for _, alrt := range dat {
        if reflect.TypeOf(alrt).Kind() == reflect.Map {

          def := newTemplate("default", alrt)
          if string(def) == "" {
            if cfg.Jiramanager.Debug { log.Print("[debug] default template is empty") }
          } else {
            if searchTask(alrt) { createTask(alrt) }
          }
        }
      }

    }

    time.Sleep(cfg.Jiramanager.Interval * time.Second)
  }
}
