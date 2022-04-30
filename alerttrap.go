package main

import (
    "net/http"
    "time"
    "log"
    "os"
    "os/signal"
    "syscall"
    "flag"
    "strings"
    "gopkg.in/natefinch/lumberjack.v2"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/ltkh/alerttrap/internal/db"
    "github.com/ltkh/alerttrap/internal/api/v1"
    "github.com/ltkh/alerttrap/internal/config"
)

var (
    cntAlerts = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "alerttrap",
            Name:      "cnt_alerts",
            Help:      "",
        },
        []string{"state","alertname"},
    )
)

func main() {

    // Command-line flag parsing
    cfFile          := flag.String("config", "config/config.yml", "config file")
    webDir          := flag.String("web-dir", "web", "site directory")
    lgFile          := flag.String("logfile", "", "log file")
    log_max_size    := flag.Int("log.max-size", 1, "log max size") 
    log_max_backups := flag.Int("log.max-backups", 3, "log max backups")
    log_max_age     := flag.Int("log.max-age", 10, "log max age")
    log_compress    := flag.Bool("log.compress", true, "log compress")
    flag.Parse()

    // Logging settings
    if *lgFile != "" {
        log.SetOutput(&lumberjack.Logger{
            Filename:   *lgFile,
            MaxSize:    *log_max_size,    // megabytes after which new file is created
            MaxBackups: *log_max_backups, // number of backups
            MaxAge:     *log_max_age,     // days
            Compress:   *log_compress,    // using gzip
        })
    }

    // Loading configuration file
    cfg, err := config.New(*cfFile)
    if err != nil {
        log.Fatalf("[error] %v", err)
    }

    // Connection to data base
    client, err := db.NewClient(cfg.Global.DB) 
    if err != nil {
        log.Fatalf("[error] connect to db: %v", err)
    }
    err = client.CreateTables()
    if err != nil {
        log.Fatalf("[error] create tables: %v", err)
    }
    // Loading alerts
    alerts, err := client.LoadAlerts()
    if err != nil {
        log.Fatalf("[error] loading alerts: %v", err)
    }
    for _, alert := range alerts {
        v1.CacheAlerts.Set(alert.GroupId, alert)
    }
    log.Printf("[info] loaded alerts from dbase (%d)", len(alerts))
    // Close connection
    client.Close()

    // Creating api
    apiV1, err := v1.New(cfg)
    if err != nil {
        log.Fatalf("[error] %v", err)
    }

    // Creating monitoring
    prometheus.MustRegister(cntAlerts)
    go func() {
        for {
            lmap := map[string]int{}
            for _, a := range v1.CacheAlerts.Items() { 
                alertname := "---"
                if val, ok := a.Labels["alertname"]; ok {
                    alertname = val.(string)
                }
                lmap[a.State+"|"+alertname] ++
            }
            for key, val := range lmap {
                spl := strings.Split(key, "|")
                cntAlerts.With(prometheus.Labels{ "state": spl[0], "alertname": spl[1] }).Set(float64(val))
            }
            time.Sleep(60 * time.Second)
        }
    }()

    // Enabled listen port
    http.HandleFunc("/-/healthy", apiV1.ApiHealthy)
    http.HandleFunc("/api/v1/sync", apiV1.ApiSync)
    http.HandleFunc("/api/v1/auth", apiV1.ApiAuth)
    http.HandleFunc("/api/v1/menu", apiV1.ApiMenu)
    http.HandleFunc("/api/v1/login", apiV1.ApiLogin)
    http.HandleFunc("/api/v1/alerts", apiV1.ApiAlerts)
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if _, err := os.Stat(*webDir+r.URL.Path); err == nil {
            http.ServeFile(w, r, *webDir+r.URL.Path)
        } else {
            http.ServeFile(w, r, *webDir+"/index.html")
        }
    })
    http.Handle("/metrics", promhttp.Handler())

    go func(cfg *config.Global){
        if cfg.Cert_file != "" && cfg.Cert_key != "" {
            if err := http.ListenAndServeTLS(cfg.Listen, cfg.Cert_file, cfg.Cert_key, nil); err != nil {
                log.Fatalf("[error] %v", err)
            }
        } else {
            if err := http.ListenAndServe(cfg.Listen, nil); err != nil {
                log.Fatalf("[error] %v", err)
            }
        }
    }(cfg.Global)

    // Program completion signal processing
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <- c
        log.Print("[info] stoping application")
        // Saving cache items
        if items := v1.CacheAlerts.Items(); len(items) != 0 {
            // Connection to data base
            client, err := db.NewClient(cfg.Global.DB) 
            if err != nil {
                log.Fatalf("[error] connect to db: %v", err)
            }
            if err := client.SaveAlerts(items); err != nil {
                log.Printf("[error] %v", err)
            }
            client.Close()
        }
        log.Print("[info] alertstrap stopped")
        os.Exit(0)
    }()

    log.Print("[info] alertstrap started -_^")

    // Delete old alerts
    go func(cfg *config.DB){
        for {
            client, err := db.NewClient(cfg) 
            if err != nil {
                log.Printf("[error] connect to db: %v", err)
            }
            // Cleaning old alerts
            cnt, err := client.DeleteOldAlerts()
            if err != nil {
                log.Printf("[error] %v", err)
            } else {
                if cnt > 0 {
                    log.Printf("[info] deleted old alerts (%d)", cnt)
                }
            }
            client.Close()

            time.Sleep(24 * time.Hour)
        }
    }(cfg.Global.DB)

    // Daemon mode
    for {

        // Mark alerts as resolved
        if keys := v1.CacheAlerts.ResolvedItems(); len(keys) != 0 {
            log.Printf("[info] alerts are marked as allowed (%d)", len(keys))
        }

        // Cleaning cache alerts
        if items := v1.CacheAlerts.ExpiredItems(); len(items) != 0 {
            client, err := db.NewClient(cfg.Global.DB)
            if err != nil {
                log.Printf("[error] connect to db: %v", err)
            }
            if err := client.SaveAlerts(items); err != nil {
                log.Printf("[error] %v", err)
            } else {
                log.Printf("[info] alerts recorded in database (%d)", len(items))
                v1.CacheAlerts.ClearItems(items)
            }
            client.Close()
        }

        time.Sleep(600 * time.Second)

    }
}
