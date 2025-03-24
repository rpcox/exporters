package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	_siteList := flag.String("site-list", "", "Location of the site list")
	//_log := flag.String("log", "", "Location of log file")
	flag.Parse()

	reg := prometheus.NewPedanticRegistry()
	ssc := NewSiteStatCollector(*_siteList)
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		ssc,
	)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Fatal(http.ListenAndServe(":9400", nil))
}
