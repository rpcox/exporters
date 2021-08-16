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

func getIPAddr(s string) string {
	index := strings.LastIndex(s, ":")
	if index == -1 {
		return s
	}

	return s[:index]
}

func getClientIP(r *http.Request) string {
	xRealIP := r.Header.Get("X-Real-Ip")
	xForwardedFor := r.Header.Get("X-Forwarded-For")

	if xRealIP == "" && xForwardedFor == "" {
		return getIPAddr(r.RemoteAddr)
	}

	if xForwardedFor != "" {
		// X-Forwarded-For is potentially a list of addresses separated with ","
		ipAddrs := strings.Split(xForwardedFor, ",")
		for i, p := range ipAddrs {
			ipAddrs[i] = strings.TrimSpace(p)
		}

		return ipAddrs[0]
	}

	return xRealIP
}

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

func GetSNGStats(w http.ResponseWriter, socket string) (int, error) {
	c, err := net.Dial("unix", socket)

	if err != nil {
		log.Print(err)
		return 0, err
	}

	defer c.Close()
	_, err = c.Write([]byte("STATS\n"))

	if err != nil {
		log.Print(err)
		return 0, err
	}

	buf := bufio.NewReader(c)
	_, err = buf.ReadString('\n')

	if err != nil {
		log.Print(err)
		return 0, err
	}

	var metricType string
	txBytes  := 0
	typeName := make(map[string]int)

	for {
		line, err := buf.ReadString('\n')

		if err != nil || line[0] == '.' { // end of STATS
			break
		}

		sngData, err := parseLine(line)
		if err != nil {
			log.Print(err)
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
			txBytes += len(typeText)
		}
		metricText := CreateMetricLine(metricName, sngData)
		fmt.Fprintln(w, metricText)
		txBytes += len(metricText)
	}

	return txBytes, nil
}

var ip, logFile, port, socket string

func init() {
	flag.StringVar(&ip,      "ip",          "0.0.0.0",                          "Server bind IP address")
	flag.StringVar(&logFile, "log-path",    "/var/log/sng-export.log",          "Logfile location")
	flag.StringVar(&port,    "port",        "8000",                             "Server bind port")
	flag.StringVar(&socket,  "socket-path", "/var/lib/syslog-ng/syslog-ng.ctl", "syslog-ng.ctl socket location")
}

func main() {

	flag.Parse()

	fhLog, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		 log.Fatal(err)
	}

	defer fhLog.Close()

	log.SetOutput(fhLog)
	log.Println("sng-export starting")
	log.Println("bind: " + ip + ":" + port)
	log.Println("syslog-ng socket: " + socket)

	rootContent :=  "<html>\n" +
			" <head><title>Syslog-NG Exporter</title></head>\n" +
			"  <body>\n" +
			"  <h1>Syslog-NG Exporter</h1>\n" +
			"  <p><a href=\"/metrics\">Metrics</a></p>\n" +
			"</body>\n" +
			"</html>"

	NFContent :=    "<html>\n" +
			" <head><title>Syslog-NG Exporter</title></head>\n" +
			"  <body>\n" +
			"  <h1>404 Not Found</h1>\n" +
			"</body>\n" +
			"</html>"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type","text/html")
		content := rootContent
		referer := r.Header.Get("Referer")
		url := r.URL.String()
		code := 200

		if referer == "" {
			referer = "-"
		}
		if url != "/" {
			code = 404
			content = NFContent
		}

		txBytes := len(content)
		fmt.Fprintln(w, content)
		log.Print(getClientIP(r), " \"" + r.Method + " " + r.URL.String() + "\" ", txBytes, " ", code, " \"" + referer + "\" " + r.Header.Get("User-Agent"))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		txBytes, err := GetSNGStats(w, socket)
		if err == nil {
			log.Print(r.Method + " \"/metrics\" ", txBytes)
		} else {
			log.Print(err)
		}
	})

	http.ListenAndServe(ip + ":" + port, mux)
}

