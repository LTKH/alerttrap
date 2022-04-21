package main

import (
    "net/http"
	"io/ioutil"
	"bytes"
	"encoding/json"
    "time"
    "log"
    "fmt"
    "math/rand"
)

type Alerts struct {
    Position     int64                     `json:"position"`
    AlertsArray  []Alert                   `json:"alerts"`
}

type Alert struct {
    AlertId      string                    `json:"alertId"`
    GroupId      string                    `json:"groupId"`
    State        string                    `json:"state"`
    Status       string                    `json:"status,omitempty"`
    StartsAt     time.Time                 `json:"startsAt"`
    EndsAt       time.Time                 `json:"endsAt"`
    Repeat       int                       `json:"repeat"`
    ChangeSt     int                       `json:"changeSt"`
    Labels       map[string]interface{}    `json:"labels"`
    Annotations  map[string]interface{}    `json:"annotations"`
    GeneratorURL string                    `json:"generatorURL"`
}

type HTTPClient struct {
    Timeout             string             `toml:"timeout"`
    Method              string             `toml:"method"`

    // HTTP Basic Auth Credentials
    Username            string             `toml:"username"`
    Password            string             `toml:"password"`

    client              *http.Client
}

func NewClient(h *HTTPClient) *HTTPClient {

    // Set default timeout
    if h.Timeout == "" {
        h.Timeout = "10s"
    }

	// Set default timeout
    if h.Method == "" {
        h.Method = "POST"
    }

    timeout, _ := time.ParseDuration(h.Timeout)

    h.client = &http.Client{
        Transport: &http.Transport{
            Proxy:           http.ProxyFromEnvironment,
        },
        Timeout: timeout,
    }

    return h
}

func (h *HTTPClient) HttpRequest(url string, data []byte) ([]byte, error) {

    req, err := http.NewRequest(h.Method, url, bytes.NewBuffer(data))
    if err != nil {
        return nil, err
    }

    if h.Username != "" || h.Password != "" {
        req.SetBasicAuth(h.Username, h.Password)
    }

    resp, err := h.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("[error] when writing to [%s] received status code: %d", url, resp.StatusCode)
    }

    if err != nil {
        return nil, fmt.Errorf("[error] when writing to [%s] received error: %v", url, err)
    }

    return body, nil
}

func main() {

    //test generate alerts
    for {
        var alerts Alerts
		for k := 0; k < 1000; k++ {
			for i := 0; i < 10; i++ {
				st := []string{"critical", "warning", "error", "resolved"}
				ri := rand.Intn(len(st))
                if k == 0 {
                    alert := Alert{
                        State:        st[ri],
                        Labels:       map[string]interface{}{
                            "alertname":   fmt.Sprintf("alertName-%d", i),
                            "node":        fmt.Sprintf("host-%d.example.com", k),
                            "tag":         fmt.Sprintf("%d", i),
                        },
                        Annotations:  map[string]interface{}{
                            "description": "test message",
                        },
                        GeneratorURL: "",
                    }
                    alerts.AlertsArray = append(alerts.AlertsArray, alert)
                } else {
                    alert := Alert{
                        State:        st[ri],
                        Labels:       map[string]interface{}{
                            "alertname":   fmt.Sprintf("alertName-%d", i),
                            "host":        fmt.Sprintf("host-%d.example.com", k),
                            "node":        fmt.Sprintf("host-%d.example.com", k),
                            "tag":         fmt.Sprintf("%d", i),
                        },
                        Annotations:  map[string]interface{}{
                            "description": "test message",
                        },
                        GeneratorURL: "",
                    }
                    alerts.AlertsArray = append(alerts.AlertsArray, alert)
                }    
			}
		}

		tmpl, err := json.Marshal(alerts)
		if err != nil {
			log.Printf("[error] %v", err)
			continue
		}
        
		client := NewClient(&HTTPClient{})
        _, err = client.HttpRequest("http://127.0.0.1:8000/api/v1/alerts", tmpl)
        if err != nil {
			log.Printf("[error] %v", err)
			continue
		}

        time.Sleep(10 * time.Second)
    }
}