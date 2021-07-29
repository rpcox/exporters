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

func TypeLine (metricName string, metricType string) string {
	slice:= []string{"# TYPE", metricName, metricType}
	return strings.Join(slice," ")
}

func MetricLine(metricName string, sng SNGData) string {
	num := fmt.Sprintf("%g", sng.value)
	s:= []string{metricName, "{sngId=\"", sng.id, "\",sngInstance=\"", sng.instance, "\",sngState=\"", sng.state, "\"} ", num}
	return strings.Join(s,"")
}

func MetricName(m SNGData) string {
	slice:= []string{"sng", m.objectType, m.statType}
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

func GetSNGStats(w http.ResponseWriter) {
	c, err := net.Dial("unix", "/var/lib/syslog-ng/syslog-ng.ctl")

	if err != nil {
		log.Print("syslog-ng.ctl connect error: ", err)
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

	var statType string
	for {
		line, err := buf.ReadString('\n')

		if err != nil || line[0] == '.' {
			// end of STATS
			break
		}

		sngData, err := parseLine(line)
		if err != nil {
			fmt.Println("parse error: ", err)
			continue
		}

		if sngData.state == "o" || sngData.state == "d" { // don't want orphans or dynamics
			continue
		}


		name := MetricName(sngData)

		switch sngData.objectType[0:4] {
		case "src.":
			switch sngData.statType[0:2] {
			case "pr": // processed
				statType = "counter"
			case "st": // stamped
				statType = "counter"
			}
		case "dst.":
			switch sngData.statType[0:1] {
			case "p", "d", "w" : // processed, dropped, written
				statType = "counter"
			case "m", "q":       // memory used, queued
				statType = "gauge"
			}
		case "filt":

		//default:
		}

	        fmt.Fprintln(w, TypeLine(name, statType))
		fmt.Fprintln(w, MetricLine(name, sngData))
	}

}

var port string

func init() {
	flag.StringVar(&port, "port", "8000", "Local port to bind")
}

func main() {

	if len(os.Args) > 2 {
		flag.Parse()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html>\n" +
			        " <head><title>Syslog-NG Exporter</title></head>\n" +
			        "  <body>\n" +
			        "  <h1>Syslog-NG Exporter</h1>\n" +
			        " <p><a href=\"/metrics\">Metrics</a></p>\n" +
			        "</body>\n" +
			        "</html>")
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		GetSNGStats(w)
	})
	http.ListenAndServe(":"+port, mux)
}

