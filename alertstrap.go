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
	"gopkg.in/natefinch/lumberjack.v2"
	"github.com/ltkh/alertstrap/internal/db"
	"github.com/ltkh/alertstrap/internal/api/v1"
	"github.com/ltkh/alertstrap/internal/config"
	"github.com/ltkh/alertstrap/internal/monitor"
)

//var logger *log.Logger

func main() {

	//limits the number of operating system threads
	runtime.GOMAXPROCS(runtime.NumCPU())

	//command-line flag parsing
	cfFile := flag.String("config", "", "config file")
	lgFile := flag.String("logfile", "", "log file") 
	//dBase  := flag.String("dbase", "", "sql file")
	flag.Parse()

	//loading configuration file
	cfg, err := config.New(*cfFile)
	if err != nil {
		log.Fatalf("[error] %v", err)
	}

	//connection to data base
	client, err := db.NewClient(&cfg.DB); 
	if err != nil {
		log.Fatalf("[error] %v", err)
	}

	//creating api
	apiV1, err := v1.New(&cfg)
	if err != nil {
		log.Fatalf("[error] %v", err)
	}

	//enabled listen port
	http.HandleFunc("/-/healthy", apiV1.ApiHealthy)
	http.HandleFunc("/api/v1/auth", apiV1.ApiAuth)
	http.HandleFunc("/api/v1/menu", apiV1.ApiMenu)
	http.HandleFunc("/api/v1/login", apiV1.ApiLogin)
	http.HandleFunc("/api/v1/alerts", apiV1.ApiAlerts)

	go func(cfg *config.Server){
		if cfg.Cert_file != "" && cfg.Cert_key != "" {
			if err := http.ListenAndServeTLS(cfg.Listen, cfg.Cert_file, cfg.Cert_key, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		} else {
			if err := http.ListenAndServe(cfg.Listen, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		}
	}(&cfg.Server)

	//opening monitoring port
	monitor.Start(cfg.Monit.Listen)

	if *lgFile != "" {
		if cfg.Server.Log_max_size == 0 {
			cfg.Server.Log_max_size = 1
		}
		if cfg.Server.Log_max_backups == 0 {
			cfg.Server.Log_max_backups = 3
		}
		if cfg.Server.Log_max_age == 0 {
			cfg.Server.Log_max_age = 28
		}
		log.SetOutput(&lumberjack.Logger{
			Filename:   *lgFile,
			MaxSize:    cfg.Server.Log_max_size,    // megabytes after which new file is created
			MaxBackups: cfg.Server.Log_max_backups, // number of backups
			MaxAge:     cfg.Server.Log_max_age,     // days
			Compress:   cfg.Server.Log_compress,    // using gzip
		})
	}

	//program completion signal processing
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<- c
		//saving cache items
		if items := v1.CacheAlerts.Items(); len(items) != 0 {
			if err := client.SaveAlerts(items); err != nil {
                log.Printf("[error] %v", err)
			}
		}
		log.Print("[info] alertstrap stopped")
		os.Exit(0)
	}()

	log.Print("[info] alertstrap started -_^")

	//delete old alerts
	go func(cfg *config.DB){
		for {
			//cleaning old alerts
			cnt, err := client.DeleteOldAlerts()
			if err != nil {
				log.Printf("[error] %v", err)
			} else {
				if cnt > 0 {
					log.Printf("[info] old alerts moved to database (%d)", cnt)
				}
			}

			time.Sleep(24 * time.Hour)
		}
	}(&cfg.DB)

	//daemon mode
	for {

		//mark alerts as resolved
		if keys := v1.CacheAlerts.ResolvedItems(); len(keys) != 0 {
            log.Printf("[info] alerts are marked as allowed (%d)", len(keys))
		}

		//cleaning cache alerts
		if items := v1.CacheAlerts.ExpiredItems(); len(items) != 0 {
			if err := client.SaveAlerts(items); err != nil {
				log.Printf("[error] %v", err)
			} else {
				log.Printf("[info] alerts recorded in database (%d)", len(items))
				v1.CacheAlerts.ClearItems(items)
			}
		}

		time.Sleep(10 * time.Second)

	}
}
