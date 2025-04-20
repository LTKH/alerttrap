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
    //"github.com/ltkh/alerttrap/internal/api/v2"
    "github.com/ltkh/alerttrap/internal/config"
)

var (
    cntAlerts = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Namespace: "alerttrap",
            Name:      "alerts",
            Help:      "",
        },
        []string{"state","alertname"},
    )
)

func main() {

    // Command-line flag parsing
    lsAddress      := flag.String("web.listen-address", ":8081", "listen address")
    webDir         := flag.String("web.dir", "web", "web directory")
    cfFile         := flag.String("config.file", "config/config.yml", "config file")
    lgFile         := flag.String("log.file", "", "log file")
    logMaxSize     := flag.Int("log.max-size", 1, "log max size") 
    logMaxBackups  := flag.Int("log.max-backups", 3, "log max backups")
    logMaxAge      := flag.Int("log.max-age", 10, "log max age")
    logCompress    := flag.Bool("log.compress", true, "log compress")
    flag.Parse()

    // Logging settings
    if *lgFile != "" {
        log.SetOutput(&lumberjack.Logger{
            Filename:   *lgFile,
            MaxSize:    *logMaxSize,    // megabytes after which new file is created
            MaxBackups: *logMaxBackups, // number of backups
            MaxAge:     *logMaxAge,     // days
            Compress:   *logCompress,   // using gzip
        })
    }

    // Loading configuration file
    cfg, err := config.New(*cfFile)
    if err != nil {
        log.Fatalf("[error] %v", err)
    }
    if cfg.Global.WebDir == "" {
        cfg.Global.WebDir = *webDir
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
    http.Handle("/metrics", promhttp.Handler())
    http.HandleFunc("/-/healthy", apiV1.ApiHealthy)
    http.HandleFunc("/api/v1/auth", apiV1.ApiAuth)
    http.HandleFunc("/api/v1/menu", apiV1.ApiMenu)
    http.HandleFunc("/api/v1/tmpl", apiV1.ApiTmpl)
    http.HandleFunc("/api/v1/login", apiV1.ApiLogin)
    http.HandleFunc("/api/v1/alerts", apiV1.ApiAlerts)
    http.HandleFunc("/api/v2/alerts", apiV1.Api2Alerts)
    http.HandleFunc("/", apiV1.ApiIndex)

    go func(cfg *config.Global){
        if cfg.CertFile != "" && cfg.CertKey != "" {
            if err := http.ListenAndServeTLS(*lsAddress, cfg.CertFile, cfg.CertKey, nil); err != nil {
                log.Fatalf("[error] %v", err)
            }
        } else {
            if err := http.ListenAndServe(*lsAddress, nil); err != nil {
                log.Fatalf("[error] %v", err)
            }
        }
    }(cfg.Global)

    // Write new proxy log
    go func(cfg *config.DB){
        // Connection to data base
        client, err := db.NewClient(cfg) 
        if err != nil {
            log.Printf("[error] connect to db: %v", err)
        }

        for prx := range apiV1.ProxyLog {
            if err := client.SaveProxyLog(*prx); err != nil {
                log.Printf("[error] %v", err)
            }
        }
    }(cfg.Global.DB)

    // Delete old records
    go func(cfg *config.DB){
        for {
            // Connection to data base
            client, err := db.NewClient(cfg) 
            if err != nil {
                log.Printf("[error] connect to db: %v", err)
            }

            // Cleaning old alerts
            clt, err := client.DeleteOldAlerts()
            if err != nil {
                log.Printf("[error] %v", err)
            } else {
                if clt > 0 {
                    log.Printf("[info] deleted old alerts (%d)", clt)
                }
            }

            // Cleaning old proxy logs
            clg, err := client.DeleteOldProxyLogs()
            if err != nil {
                log.Printf("[error] %v", err)
            } else {
                if clg > 0 {
                    log.Printf("[info] deleted old proxy log (%d)", clg)
                }
            }

            client.Close()
            time.Sleep(24 * time.Hour)
        }
    }(cfg.Global.DB)

    // Program signal processing
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
    go func(){
        for {
            s := <-c
            switch s {
                case syscall.SIGHUP:
                    // Loading configuration file
                    cfg, err = config.New(*cfFile)
                    if err != nil {
                        log.Printf("[error] %v", err)
                    } else {
                        _, err := v1.New(cfg)
                        if err != nil {
                            log.Fatalf("[error] %v", err)
                        } else {
                            log.Print("[info] reload happened")
                        }
                    } 
                default:
                    log.Print("[info] application stoping")
                    if items := v1.CacheAlerts.Items(); len(items) != 0 {
                        // Connection to data base
                        client, err := db.NewClient(cfg.Global.DB) 
                        if err != nil {
                            log.Fatalf("[error] connect to db: %v", err)
                        }
                        // Saving cache items
                        if err := client.SaveAlerts(items); err != nil {
                            log.Printf("[error] %v", err)
                        }
                        client.Close()
                    }
                    log.Print("[info] alerttrap stopped")
                    os.Exit(0)
            }
        }
    }()

    log.Print("[info] alerttrap started -_^")

    // Daemon mode
    for {

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
