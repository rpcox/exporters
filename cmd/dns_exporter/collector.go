package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
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
	HttpRequestAttemptsTotal *prometheus.CounterVec
	HttpRequestSuccessTotal  *prometheus.CounterVec
	HttpDuration             *prometheus.GaugeVec
	HttpDurationBucket       *prometheus.HistogramVec
	siteListFile             string
	sites                    *[]string
}

var siteReloadSignal bool

// Read a tab delimited text file with a header
// File format - tab separated
//
//	0           1          2         3         4         5          6
//
// ENDPOINT \t METHOD \t AUTHTYPE \t USER \t PASSWORD \t ACCEPT \t PAYLOAD
func loadSites(fileName string) (*[]string, error) {
	var list []string
	if fh, err := os.Open(fileName); err == nil {
		defer fh.Close()

		reader := csv.NewReader(fh)
		reader.Comment = '#'
		reader.Comma = '\t'

		rows, err := reader.ReadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fileName, err)
			return nil, err
		}

		for _, row := range rows {
			list = append(list, row[0])
		}

	} else {
		fmt.Fprintf(os.Stderr, "%s: %v\n", fileName, err)
		return nil, err
	}

	return &list, nil
}

func NewSiteStatCollector(siteList string) prometheus.Collector {
	siteReloadSignal = false

	ssc := SiteStatCollector{}
	ssc.siteListFile = siteList
	S, err := loadSites(ssc.siteListFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s: %v\n", siteList, err)
		os.Exit(2)
	}
	ssc.sites = S

	ssc.HttpRequestAttemptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dns_lookup_attempt_total",
		Help: "Total number of DNS A record requests partitioned by site",
	},
		[]string{"site"},
	)

	ssc.HttpRequestSuccessTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dns_lookup_success_total",
		Help: "Total number of DNS A record requests partitioned by site and IP address",
	},
		[]string{"site", "ip"},
	)

	ssc.HttpDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dns_lookup_duration_seconds",
		Help: "Duration of DNS A record requests partitioned by site",
	},
		[]string{"site"},
	)

	ssc.HttpDurationBucket = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "dns_duration_seconds",
		Help:    "Duration of DNS A record requests partitioned by site",
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
	for _, site := range *ssc.sites {

		if c, err := ssc.HttpRequestAttemptsTotal.GetMetricWithLabelValues(site); err == nil {
			c.Inc()
		} else {
			log.Println(err)
		}

		start := time.Now()
		IpList, err := net.LookupHost(site)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}
		duration := time.Since(start).Seconds()

		if c, err := ssc.HttpRequestSuccessTotal.GetMetricWithLabelValues(site, IpList[0]); err == nil {
			c.Inc()
		} else {
			log.Println(err)
		}

		if d, err := ssc.HttpDuration.GetMetricWithLabelValues(site); err == nil {
			d.Set(float64(duration))
		} else {
			log.Println(err)
		}

		g, err := ssc.HttpDurationBucket.GetMetricWithLabelValues(site)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			g.Observe(float64(duration))
		}
	}
	/*
		if siteReloadSignal {
			newLoad := loadSites(ssc.siteListFile)
			if newLoad != nil {
				ssc.sites = newLoad
				log.Printf("success: loaded %d sites\n", len(*newLoad))
			} else {
				log.Printf("fail: received nil list \n")
			}
			siteReloadSignal = false
		}
	*/
}
