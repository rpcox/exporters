package main

import (
	"encoding/csv"
	"log"
	"net/http"
	"net/url"
	"os"
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
	Accept   string
	Payload  string
}

type SiteStatCollector struct {
	Sites              *[]Site
	HttpAuthReqTotal   *prometheus.CounterVec
	HttpReqTotal       *prometheus.CounterVec
	HttpDuration       *prometheus.GaugeVec
	HttpDurationBucket *prometheus.HistogramVec
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
			p, err := url.Parse(row[0])
			if err != nil {
				log.Fatal(err)
			}

			// File format - tab separated
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

	// add a format check here later
	/*	for _, site := range list {
			fmt.Println("*** Endpoint:", site.EndPoint)
			fmt.Println("Host", site.Host)
			fmt.Println("Method", site.Method)
			fmt.Println("AuthType", site.AuthType)
			fmt.Println("User", site.User)
			fmt.Println("Password", site.Password)
			fmt.Println("Accept", site.Accept)
			fmt.Println("Payload", site.Payload)
		}
		os.Exit(0) */
	return &list
}

func NewSiteStatCollector(siteList string) prometheus.Collector {
	sites := loadSites(siteList)

	fqname := func(name string) string {
		return "scc_" + name
	}

	_httpReqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: fqname("http_requests_total"),
		Help: "Total number of requests by site",
	},
		[]string{"site"},
	)

	_httpAuthReqTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: fqname("http_auth_requests_total"),
		Help: "Total number of requests by site and HTTP status code",
	},
		[]string{"site", "code"},
	)

	_httpDuration := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: fqname("_http_request_duration"),
		Help: "Duration of HTTP requests",
	},
		[]string{"site"},
	)
	_httpDurationHist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    fqname("http_request_duration_bucket"),
		Help:    "Duration of HTTP requests",
		Buckets: []float64{0.01, 0.1, 0.5, 1, 2, 5},
	},
		[]string{"site"},
	)

	prometheus.MustRegister(_httpReqTotal)
	prometheus.MustRegister(_httpAuthReqTotal)
	prometheus.MustRegister(_httpDuration)
	prometheus.MustRegister(_httpDurationHist)

	return &SiteStatCollector{
		Sites:              sites,
		HttpAuthReqTotal:   _httpAuthReqTotal,
		HttpReqTotal:       _httpReqTotal,
		HttpDuration:       _httpDuration,
		HttpDurationBucket: _httpDurationHist,
	}
}

func (ssc *SiteStatCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(ssc, ch)
}

func (ssc *SiteStatCollector) Collect(ch chan<- prometheus.Metric) {
	for _, site := range *ssc.Sites {
		// One could make this call below, however, the call panics where
		// GetMetricWithLabelValues() returns an error to handle
		// 	ssc.HttpReqTotal.WithLabelValues(site.Host).Inc()
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
		defer resp.Body.Close()
		/*		statusCode := strconv.Itoa(resp.StatusCode)

				if c, err := ssc.HttpAuthReqTotal.GetMetricWithLabelValues(site.Host, statusCode); err == nil {
					c.Inc()
					ch <- c
				} else {
					log.Println(err)
				}
		*/
		if g, err := ssc.HttpDuration.GetMetricWithLabelValues(site.Host); err == nil {
			g.Set(duration)
			ch <- g
		} else {
			log.Println(err)
		}

		if h, err := ssc.HttpDurationBucket.GetMetricWithLabelValues(site.Host); err == nil {
			h.Observe(duration)
			ssc.HttpDurationBucket.Collect(ch)
		} else {
			log.Println(err)
		}
	}
}
