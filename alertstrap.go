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
  "github.com/ltkh/alertstrap/internal/db"
  "github.com/ltkh/alertstrap/internal/api"
  "github.com/ltkh/alertstrap/internal/config"
  "github.com/ltkh/alertstrap/internal/monitor"
)

func main() {

	//limits the number of operating system threads
	runtime.GOMAXPROCS(runtime.NumCPU())

	//command-line flag parsing
	cfFile := flag.String("config", "", "config file")
	//dBase  := flag.String("dbase", "", "sql file")
	flag.Parse()

	//loading configuration file
	cfg, err := config.LoadConfigFile(*cfFile)
	if err != nil {
		log.Fatalf("[error] %v", err)
	}

	//connection to data base
	log.Print("[info] connection to data base")
	if err := db.ConnectDb(cfg); err != nil {
		log.Fatalf("[error] %v", err)
	}

	//creating data base schema
	//if *dBase != "" {
	//  log.Print("[info] creating data base schema")
	//  if err := db.CreateSchema(*dBase); err != nil {
	//    log.Fatalf("[critical] %v", err)
	//  }
	//}

	//loading alerts
	log.Print("[info] loading alerts from database")
	if err := api.LoadAlerts(); err != nil {
		log.Fatalf("[error] %v", err)
	}

	//opening monitoring port
	monitor.Start(cfg.Monit.Listen)

	//loading hosts table
	//log.Print("[info] loading hosts from database")
	//if err := api.LoadHosts(); err != nil {
	//  log.Fatalf("[critical] %v", err)
	//}

	//program completion signal processing
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<- c
		//saving cache items
		if items := api.Cache.Items(); len(items) != 0 {
			db.SaveItems(items)
		}
		log.Print("[info] alertstrap stopped")
		os.Exit(0)
	}()

	//enabled listen port
	http.HandleFunc("/v1/login", api.Login)
	http.HandleFunc("/v1/get/alerts", api.GetAlerts)
	http.HandleFunc("/v1/add/alerts", api.AddAlerts)
	
	//http.HandleFunc("/api/v1/get/history", api.GetHistory)
	//go http.ListenAndServe(cfg.Server.Listen_port, nil)

	go func(cfg *config.Config){
		if cfg.Server.Cert_file != "" && cfg.Server.Cert_key != "" {
			if err := http.ListenAndServeTLS(cfg.Server.Listen, cfg.Server.Cert_file, cfg.Server.Cert_key, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		} else {
			if err := http.ListenAndServe(cfg.Server.Listen, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		}
		log.Printf("[info] listen port enabled - %s", cfg.Server.Listen)
	}(&cfg)

	log.Print("[info] alertstrap started ^_^")

	//daemon mode
	for {

		//connection to data base
		if err := db.ConnectDb(cfg); err != nil {
			log.Printf("[error] %v", err)
		}

		//cleaning cache items
		if items := api.Cache.ExpiredItems(); len(items) != 0 {
			db.SaveItems(items)
			api.Cache.ClearItems(items)
		}

		//loading alerts
		//if err := api.LoadAlerts(); err != nil {
		//  log.Fatalf("[critical] %v", err)
		//}

		//if err := db.ConnectDb(cfg); err != nil {
		//  log.Printf("[error] %v", err)
		//  db.ConnectDb(cfg)
		//} else {
		//  if err := api.LoadHosts(); err != nil {
		//    log.Printf("[error] %v", err)
		//  }
		//}

		time.Sleep(10 * time.Second)

	}
}
