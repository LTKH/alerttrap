package main

import (
  "net/http"
  "time"
  "log"
  "os"
  "os/signal"
  "syscall"
  //"fmt"
  //"html"
  "runtime"
  "flag"
  "alertstrap/db"
  "alertstrap/api"
  "alertstrap/config"
)

func main() {

  runtime.GOMAXPROCS(runtime.NumCPU())

  cfFile := flag.String("config", "", "config file")
  flag.Parse()

  cfg, err := config.LoadConfigFile(*cfFile)
  if err != nil {
    log.Fatalf("[error] v%", err)
  }

  db.Conn = db.ConnectDb(cfg)
  if db.Conn != nil {
    db.CreateSchema()
    api.LoadAlerts()
  }

  go http.ListenAndServe(cfg.Showcase.Listen_port, &(api.Api{ Cfg: cfg }))

  log.Print("[info] showcase started ^_^")

  c := make(chan os.Signal, 2)
  signal.Notify(c, os.Interrupt, syscall.SIGTERM)
  go func() {
    <- c
    log.Print("[info] showcase stopped")
    os.Exit(0)
  }()

  for {
    time.Sleep(10 * time.Second)

    if db.Conn == nil {
      db.Conn = db.ConnectDb(cfg)
      if db.Conn != nil {
        api.LoadAlerts()
      }
    }
  }
}
