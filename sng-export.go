// sng_export.go - Syslog-NG exporter for Prometheus
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type SNGData struct {      // Field names from 'syslog-ng-ctl stats' call
	objectType string  // SourceName
	id         string  // SourceId
	instance   string  // SourceInstance
	state      string  // State (a, d, o)
	statType   string  // Type (dropped, processed, ...)
	value      float64 // Number
}

func CreateTypeLine (metricName string, metricType string) string {
	slice:= []string{"# TYPE", metricName, metricType}
	return strings.Join(slice," ")
}

func CreateMetricLine(metricName string, sng SNGData) string {
	num := fmt.Sprintf("%g", sng.value)
	if sng.instance == "" {
		host, err := os.Hostname()
		if err == nil {
			sng.instance = host
		}
	}
	s:= []string{metricName, "{id=\"", sng.id, "\",sng_instance=\"", sng.instance, "\",state=\"", sng.state, "\"} ", num}
	return strings.Join(s,"")
}

func CreateMetricName(m SNGData, st string) string {
	var slice []string

	switch st[0:1] {
		case "c":                    // counter
			slice = []string{"sng", m.objectType, m.statType, "total"}
		case "g":                    // gauge
			slice = []string{"sng", m.objectType, m.statType}
	}

	return strings.ReplaceAll(strings.Join(slice,"_"), ".", "_")
}


func parseLine(line string) (SNGData, error) {
	var s SNGData
	chunk := strings.SplitN(strings.TrimSpace(line), ";", 6)
	num, err := strconv.ParseFloat(chunk[5], 64)

	if err != nil {
		return s, err
	}

	s = SNGData{chunk[0], chunk[1], chunk[2], chunk[3], chunk[4], num}
	return s, nil
}

func GetSNGStats(w http.ResponseWriter, socket string) {
	c, err := net.Dial("unix", socket)

	if err != nil {
		log.Print("syslog-ng.ctl Dial() error: ", err)
		return
	}

	defer c.Close()
	_, err = c.Write([]byte("STATS\n"))

	if err != nil {
		log.Print("syslog-ng.ctl write error: ", err)
		return
	}

	buf := bufio.NewReader(c)
	_, err = buf.ReadString('\n')

	if err != nil {
		log.Print("syslog-ng.ctl read error: ", err)
		return
	}

	var metricType string
	typeName := make(map[string]int)

	for {
		line, err := buf.ReadString('\n')

		if err != nil || line[0] == '.' { // end of STATS
			break
		}

		sngData, err := parseLine(line)
		if err != nil {
			log.Print("parse error: ", err)
			continue
		}

		if sngData.state == "o" || sngData.state == "d" { // don't want orphans or dynamics
			continue
		}


		switch sngData.statType[0:2] {
			case "co", "me", "qu" :  // connections, memory_usage, queued
				metricType = "gauge"
			default:                 // dropped, matched, not_matched, processed, stamp, value, written
				metricType = "counter"
		}

		metricName := CreateMetricName(sngData, metricType)
		typeText := CreateTypeLine(metricName, metricType)
		_, exist := typeName[metricName]

		if ! exist {
			typeName[metricName] = 1
			fmt.Fprintln(w, typeText)
		}

		fmt.Fprintln(w, CreateMetricLine(metricName, sngData))
	}
}

var port, logFile, socket string

func init() {
	flag.StringVar(&port,    "port",        "8000",                             "Server bind port")
	flag.StringVar(&socket,  "socket-path", "/var/lib/syslog-ng/syslog-ng.ctl", "syslog-ng.ctl socket location")
	flag.StringVar(&logFile, "log-path",    "/var/log/sng-export.log",          "Logfile location")
}

func main() {

	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type","text/html")
		fmt.Fprintln(w, "<html>\n" +
			        " <head><title>Syslog-NG Exporter</title></head>\n" +
			        "  <body>\n" +
			        "  <h1>Syslog-NG Exporter</h1>\n" +
			        " <p><a href=\"/metrics\">Metrics</a></p>\n" +
			        "</body>\n" +
			        "</html>")
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		GetSNGStats(w, socket)
	})
	http.ListenAndServe(":"+port, mux)
}

