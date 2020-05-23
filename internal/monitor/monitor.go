package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"github.com/ltkh/alertstrap/internal/api/v1"
	"time"
)

var (
	cntAlerts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "alertstrap",
			Name:      "cnt_alerts",
			Help:      "",
		},
		[]string{/*"status","alertname","node"*/},
	)
)

func Start(Listen string) {

	http.Handle("/metrics", promhttp.Handler())

	prometheus.MustRegister(cntAlerts)

	go func() {
		for {
			items := v1.CacheAlerts.Items()
            //lmap := map[string]string
			
			//for _, val := range items {
			//	val.Value.Labels
				
			//}
            cntAlerts.With(prometheus.Labels{}).Set(float64(len(items)))

			//labels := []string{}

			/*
			for key, _ := range api.Cache.Items() {
				addAlerts.With(prometheus.Labels{"location": key}).Set(float64(1))
			}
			for key, _ := range streams.Job_chan {
				jobGauge.With(prometheus.Labels{"location": key}).Set(float64(len(streams.Job_chan[key])))
			}
			for key, limit := range streams.Stt_stat {
				limit.Stat.Range(func(k, v interface{}) bool {
					sttGauge.With(prometheus.Labels{"limit": key, "key": k.(string)}).Set(float64(v.(int)))
					streams.Stt_stat[key].Stat.Store(k, 0)
					return true
				})
			}
			*/
			time.Sleep(60 * time.Second)
		}
	}()

	go http.ListenAndServe(Listen, nil)
}