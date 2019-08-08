package main

import (
  "net/http"
  "time"
  "log"
  "os"
  "os/signal"
  "syscall"
  "runtime"
  "flag"
  //"alertstrap/db"
  "alertstrap/api"
  "alertstrap/config"
)

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

  //cache initialization
  //api.StoreInit()

  //database connection
  //db.Conn = db.ConnectDb(cfg)
  //if db.Conn != nil {
  //  db.CreateSchema()
  //  api.LoadAlerts()
  //}

  //opening port for requests
  go http.ListenAndServe(cfg.Alertstrap.Listen_port, &(api.Api{ Cfg: cfg }))

  log.Print("[info] alertstrap started ^_^")

  //program completion signal processing
  c := make(chan os.Signal, 2)
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)
  go func() {
    <- c
    log.Print("[info] alertstrap stopped")
    os.Exit(0)
  }()

  //daemon mode
  for {
    time.Sleep(10 * time.Second)

    //if db.Conn == nil {
    //  db.Conn = db.ConnectDb(cfg)
    //  if db.Conn != nil {
    //    api.LoadAlerts()
    //  }
    //}
  }
}
