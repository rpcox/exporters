package main

import (
	"encoding/csv"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Site struct {
	EndPoint string // e.g. https://www.spud.com:5000/api/v2/
	Host     string
	Method   string
	AuthType string
	User     string
	Password string
	Payload  string
}

type SiteStatCollector struct {
	Sites            *[]Site
	HttpAuthReqTotal *prometheus.CounterVec
	HttpReqTotal     *prometheus.CounterVec
	HttpDuration     *prometheus.GaugeVec
}

// Read a tab delimited text file with a header
// Endpoint  Method  AuthType  User  Password Payload
func loadSites(siteList string) *[]Site {
	var list []Site
	if fh, err := os.Open(siteList); err == nil {
		defer fh.Close()

		reader := csv.NewReader(fh)
		reader.Comma = '\t'
		reader.Read() // remove the header

		rows, err := reader.ReadAll()
		if err != nil {
			log.Fatal("cannot read site list:", siteList)
		}

		for _, row := range rows {
			site := Site{
				EndPoint: row[0],
				Method:   row[1],
				AuthType: row[2],
				User:     row[3],
				Password: row[4],
				Payload:  row[5],
			}

			list = append(list, site)
		}

	} else {
		log.Fatal("cannot open site list:", siteList)
	}

	return &list
}

func NewSiteStatCollector(siteList string) prometheus.Collector {
	sites := loadSites(siteList)

	_httpReqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ssc_http_requests_total",
		Help: "Total number of requests by site",
	},
		[]string{"site"},
	)

	_httpAuthReqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ssc_http_auth_requests_total",
		Help: "Total number of requests by site and HTTP status code",
	},
		[]string{"site", "code"},
	)

	_httpDuration := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ssc_http_request_duration",
		Help: "Duration of HTTP requests",
	},
		[]string{"site"},
	)
	prometheus.MustRegister(_httpReqTotal)
	prometheus.MustRegister(_httpAuthReqTotal)
	prometheus.MustRegister(_httpDuration)

	return &SiteStatCollector{
		Sites:            sites,
		HttpAuthReqTotal: _httpAuthReqTotal,
		HttpReqTotal:     _httpReqTotal,
		HttpDuration:     _httpDuration,
	}
}

func (ssc *SiteStatCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ssc, ch)
}

func (ssc *SiteStatCollector) Collect(ch chan<- prometheus.Metric) {
	for _, site := range *ssc.Sites {
		if c, err := ssc.HttpReqTotal.GetMetricWithLabelValues(site.Host); err == nil {
			c.Inc()
			ch <- c
		} else {
			log.Println(err)
		}

		req, err := http.NewRequest(site.Method, site.EndPoint, nil)
		if err != nil {
			log.Println(err)
			continue
		}
		req.Header.Add(`Accept`, `application\json`)
		if site.User != "" {
			req.SetBasicAuth(site.User, site.Password)
		}

		client := http.Client{
			Timeout: time.Duration(1) * time.Minute,
		}

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			continue
		}
		duration := time.Since(start).Seconds()
		defer resp.Body.Close()
		statusCode := strconv.Itoa(resp.StatusCode)

		if c, err := ssc.HttpAuthReqTotal.GetMetricWithLabelValues(site.Host, statusCode); err == nil {
			c.Inc()
			ch <- c
		} else {
			log.Println(err)
		}

		if g, err := ssc.HttpDuration.GetMetricWithLabelValues(site.Host); err == nil {
			g.Set(duration)
			ch <- g
		} else {
			log.Println(err)
		}
	}
}
