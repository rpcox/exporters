package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	_siteList := flag.String("site-list", "", "Location of the site list")
	//_log := flag.String("log", "", "Location of log file")
	flag.Parse()

	ssc := NewSiteStatCollector(*_siteList)
	prometheus.Register(ssc)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9400", nil))
}
