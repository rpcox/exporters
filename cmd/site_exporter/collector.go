package main

import (
	"encoding/csv"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Site struct {
	EndPoint string // e.g. https://www.spud.com:5000/api/v2/abc/def/?=xyz
	Host     string
	Method   string
	AuthType string
	User     string
	Password string
	Accept   string
	Payload  string
}

type SiteStatCollector struct {
	Sites                    *[]Site
	HttpRequestAttemptsTotal *prometheus.CounterVec
	HttpRequestSuccessTotal  *prometheus.CounterVec
	HttpDuration             *prometheus.GaugeVec
	HttpDurationBucket       *prometheus.HistogramVec
}

// Read a tab delimited text file with a header
// File format - tab separated
//
//	0           1          2         3         4         5          6
//
// ENDPOINT \t METHOD \t AUTHTYPE \t USER \t PASSWORD \t ACCEPT \t PAYLOAD
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
			p, err := url.Parse(row[0])
			if err != nil {
				log.Fatal(err)
			}

			//   0           1          2         3         4         5          6
			// ENDPOINT \t METHOD \t AUTHTYPE \t USER \t PASSWORD \t ACCEPT \t PAYLOAD
			site := Site{
				EndPoint: row[0],
				Host:     p.Host,
				Method:   row[1],
				AuthType: row[2],
				User:     row[3],
				Password: row[4],
				Accept:   row[5],
				Payload:  row[6],
			}

			list = append(list, site)
		}

	} else {
		log.Fatal("cannot open site list:", siteList)
	}

	return &list
}

func NewSiteStatCollector(siteList string) prometheus.Collector {
	ssc := SiteStatCollector{}
	ssc.Sites = loadSites(siteList)

	ssc.HttpRequestAttemptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_attempt_total",
		Help: "Total number of requests partitioned by site",
	},
		[]string{"site"},
	)

	ssc.HttpRequestSuccessTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_success_total",
		Help: "Total number of requests partitioned by site and HTTP status code",
	},
		[]string{"site", "code"},
	)

	ssc.HttpDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests",
	},
		[]string{"site"},
	)

	ssc.HttpDurationBucket = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds_bucket",
		Help:    "Duration of HTTP requests bucketed by histogram and partitioned by site",
		Buckets: []float64{0.001, 0.01, 0.1, 0.25, 0.5, 1, 2, 5},
	},
		[]string{"site"},
	)

	prometheus.MustRegister(ssc.HttpRequestAttemptsTotal)
	prometheus.MustRegister(ssc.HttpRequestSuccessTotal)
	prometheus.MustRegister(ssc.HttpDuration)
	prometheus.MustRegister(ssc.HttpDurationBucket)

	return &ssc
}

func (ssc *SiteStatCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ssc, ch)
}

func (ssc *SiteStatCollector) Collect(ch chan<- prometheus.Metric) {
	for _, site := range *ssc.Sites {

		if c, err := ssc.HttpRequestAttemptsTotal.GetMetricWithLabelValues(site.Host); err == nil {
			c.Inc()
		} else {
			log.Println(err)
		}

		req, err := http.NewRequest(site.Method, site.EndPoint, nil)
		if err != nil {
			log.Println(err)
			continue
		}
		req.Header.Add(`Accept`, site.Accept)
		if site.AuthType == "basic" {
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
		statusCode := strconv.Itoa(resp.StatusCode)
		resp.Body.Close()

		if d, err := ssc.HttpDuration.GetMetricWithLabelValues(site.Host); err == nil {
			d.Set(float64(duration))
		} else {
			log.Println(err)
		}

		if c, err := ssc.HttpRequestSuccessTotal.GetMetricWithLabelValues(site.Host, statusCode); err == nil {
			c.Inc()
		} else {
			log.Println(err)
		}

		g, err := ssc.HttpDurationBucket.GetMetricWithLabelValues(site.Host)
		if err != nil {
			log.Println(err)
		} else {
			g.Observe(float64(duration))
		}
	}
}
