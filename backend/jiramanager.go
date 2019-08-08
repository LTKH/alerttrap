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

func newRequest(cfg config.Config, method string, url string, jsonStr []byte, login string, password string) ([]byte, error) {
  if cfg.Jiramanager.Debug { log.Printf("[debug] -- new request (%v)", url) }

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
    return nil, err
  }

  return body, nil

}

func newTemplate(cfg config.Config, name string, vals interface{}) []byte {
  if cfg.Jiramanager.Debug { log.Printf("[debug] -- create new template (%v)", name) }

  tmpl, err := template.ParseFiles(cfg.Jiramanager.Tmpl_dir+"/"+name+".tmpl")
  if err != nil {
    log.Printf("[error] %v", err)
    return []byte("")
  }

  var tpl bytes.Buffer
  if err = tmpl.Execute(&tpl, &vals); err != nil {
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

func searchTask(cfg config.Config, tmpl string, alrt map[string]interface{}) bool {
  if cfg.Jiramanager.Debug { log.Printf("[debug] -- serching old task (%v)", tmpl) }

  alrt["task_key"] = db.GetTaskKey(alrt["mgrp_id"].(string))
  if alrt["task_key"] == "" {
    if cfg.Jiramanager.Debug { log.Print("[debug] task_key is empty") }
    return false
  }

  def := newTemplate(cfg, tmpl, alrt)
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(def)) }

  sch, err := newRequest(cfg, "POST", cfg.Jiramanager.Jira_api+"/search", def, cfg.Jiramanager.Login, cfg.Jiramanager.Passwd)
  if err != nil {
    if cfg.Jiramanager.Debug { log.Printf("[debug] %v", err) }
    return true
  }
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(sch)) }

  js, err := parseJson(sch)
  if err != nil {
    log.Printf("[error] %v", err)
    return true
  }

  if js["issues"] != nil {
    arr := js["issues"].([]interface{})
    if len(arr) > 0 {
      return true
    }
  }

  return false
}

func createTask(cfg config.Config, tmpl string, alrt map[string]interface{}) bool {

  def := newTemplate(cfg, tmpl, alrt)
  if string(def) == "" {
    log.Printf("[warn] default template is empty (%v)", alrt["mgrp_id"].(string))
    return false
  }
  if cfg.Jiramanager.Debug { log.Printf("[debug] %v", string(def)) }

  if cfg.Jiramanager.Search {
    if searchTask(cfg, "search", alrt) {
      log.Printf("[info] task already exists (%v)", alrt["mgrp_id"].(string))
      return false
    }
  }

  _, err := parseJson(def)
  if err != nil {
    log.Printf("[error] %v", err)
    return false
  }

  crt, err := newRequest(cfg, "POST", cfg.Jiramanager.Jira_api+"/issue", def, cfg.Jiramanager.Login, cfg.Jiramanager.Passwd)
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

  log.Print("[info] alertsender running ^_-")

  //program completion signal processing
  c := make(chan os.Signal, 2)
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)
  go func() {
    <- c
    log.Print("[info] alertsender stopped")
    os.Exit(0)
  }()

  //daemon mode
  for {

    //database connection
    db.Conn = db.ConnectDb(cfg)
    defer db.Conn.Close()

    //
    body, err := newRequest(cfg, "GET", cfg.Jiramanager.Get_alerts, []byte(""), "", "")
    if err != nil {
      log.Printf("[error] %v", err)
    } else {

      //
      if cfg.Jiramanager.Debug { log.Print("[debug] parsing alerts") }
      var dat []map[string]interface{}
      if err := json.Unmarshal(body, &dat); err != nil {
          log.Printf("[error] %v", err)
      }

      //
      for _, alrt := range dat {
        if reflect.TypeOf(alrt).Kind() == reflect.Map {
          createTask(cfg, "default", alrt)
        }
      }
    }

    time.Sleep(cfg.Jiramanager.Interval * time.Second)
  }
}
