package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"github.com/ltkh/alerttrap/internal/api/v1"
	"time"
	//"log"
	"strings"
)

var (
	cntAlerts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "alerttrap",
			Name:      "cnt_alerts",
			Help:      "",
		},
		[]string{"state","alertname","node"},
	)
)

func Start(Listen string) {

	http.Handle("/metrics", promhttp.Handler())

	prometheus.MustRegister(cntAlerts)

	go func() {
		for {
			
            lmap := map[string]int{}
			
			for _, a := range v1.CacheAlerts.Items() { 
				alertname := ""
				node      := ""
				for key, val := range a.Labels {
                    if key == "alertname" {
						alertname = val.(string)
					}
					if key == "node" {
						node = val.(string)
					}
				}
				lmap[a.State+"|"+alertname+"|"+node] ++
			}

			for key, val := range lmap {
				spl := strings.Split(key, "|")
				cntAlerts.With(prometheus.Labels{ "state": spl[0], "alertname": spl[1], "node": spl[2] }).Set(float64(val))
			}

			time.Sleep(60 * time.Second)
		}
	}()

	go http.ListenAndServe(Listen, nil)
}