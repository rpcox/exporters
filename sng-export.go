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

func logIt(r *http.Request, txBytes int, code int) {

	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "-"
	}

	log.Print(getClientIP(r), " \""+r.Method+" "+r.URL.String()+"\" ", txBytes, " ", code, " \""+referer+"\" "+r.Header.Get("User-Agent"))
}

// Field names from 'syslog-ng-ctl stats' call
// objectType;id;instance;state;statType;value
//
// Examples
// dst.file;d_host#0;/mnt/data/log/192.168.1.15/20211211-192.168.1.15-systemd.log;o;written;1
// global;msg_clones;;a;processed;0
// destination;d_junk;;a;processed;0
// src.tcp;s_net#0;tcp,192.168.1.150;a;processed;34
//
type SNGData struct {
	objectType string  // SourceName
	id         string  // SourceId
	instance   string  // SourceInstance
	state      string  // State (a, d, o)
	statType   string  // Type (dropped, processed, ...)
	value      float64 // Number
}

func CreateTypeLine(metricName string, metricType string) string {
	slice := []string{"# TYPE", metricName, metricType}
	return strings.Join(slice, " ")
}

func CreateMetricLine(metricName string, sng SNGData) string {
	num := fmt.Sprintf("%g", sng.value)
	s := []string{metricName, "{id=\"", sng.id, "\",sng_instance=\"", sng.instance, "\",state=\"", sng.state, "\"} ", num}
	return strings.Join(s, "")
}

func CreateMetricName(m SNGData, st string) string {
	var slice []string

	switch st[0:1] {
	case "c": // counter
		slice = []string{"sng", m.objectType, m.statType, "total"}
	case "g": // gauge
		slice = []string{"sng", m.objectType, m.statType}
	}

	return strings.ReplaceAll(strings.Join(slice, "_"), ".", "_")
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
	typeText := "# TYPE sng_dial_state gauge"
	fmt.Fprintln(w, typeText)
	txBytes := len(typeText)

	status := "sng_net_dial{id=\"status_metric\"}"
	txBytes += len(status)

	c, err := net.Dial("unix", socket)

	if err != nil {
		fmt.Fprintln(w, status, "0")
		log.Print(err)
		return txBytes + 2, err
	}

	defer c.Close()
	fmt.Fprintln(w, status, "1")
	txBytes += 2

	typeText = "# TYPE sng_dial_state gauge"
	fmt.Fprintln(w, typeText)
	txBytes += len(typeText)
	status = "sng_socket_write{id=\"status_metric\"}"
	txBytes += len(status)

	_, err = c.Write([]byte("STATS\n"))

	if err != nil {
		fmt.Fprintln(w, status, "0")
		log.Print(err)
		return txBytes + 2, err
	}

	fmt.Fprintln(w, status, "1")
	txBytes += 2

	typeText = "# TYPE sng_buffer_state gauge"
	fmt.Fprintln(w, typeText)
	txBytes += len(typeText)
	status = "sng_buffer_read{id=\"status_metric\"}"
	txBytes += len(status)

	buf := bufio.NewReader(c)
	_, err = buf.ReadString('\n')

	if err != nil {
		fmt.Fprintln(w, status, "0")
		log.Print(err)
		return txBytes + 2, err
	}

	fmt.Fprintln(w, status, "1")
	txBytes += 2

	var metricType string
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
		case "co", "me", "qu": // connections, memory_usage, queued
			metricType = "gauge"
		default: // dropped, matched, not_matched, processed, stamp, value, written
			metricType = "counter"
		}

		metricName := CreateMetricName(sngData, metricType)
		typeText = CreateTypeLine(metricName, metricType)
		_, exist := typeName[metricName]

		// if the typeName has not come up yet, write it
		if !exist {
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
	flag.StringVar(&ip, "ip", "0.0.0.0", "Server bind IP address")
	flag.StringVar(&logFile, "log-path", "/var/log/sng-export.log", "Logfile location")
	flag.StringVar(&port, "port", "8000", "Server bind port")
	flag.StringVar(&socket, "socket-path", "/var/lib/syslog-ng/syslog-ng.ctl", "syslog-ng.ctl socket location")
}

func main() {

	flag.Parse()

	fhLog, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	defer fhLog.Close()

	log.SetOutput(fhLog)
	log.SetFlags(log.Lmicroseconds | log.LUTC | log.Ldate | log.Ltime)
	log.Println("sng-export starting")
	log.Println("bind: " + ip + ":" + port)
	log.Println("syslog-ng socket: " + socket)

	rootContent := "<html>\n" +
		" <head><title>Syslog-NG Exporter</title></head>\n" +
		"  <body>\n" +
		"  <h1>Syslog-NG Exporter</h1>\n" +
		"  <p><a href=\"/metrics\">Metrics</a></p>\n" +
		"</body>\n" +
		"</html>"

	NFContent := "<html>\n" +
		" <head><title>Syslog-NG Exporter</title></head>\n" +
		"  <body>\n" +
		"  <h1>404 Not Found</h1>\n" +
		"</body>\n" +
		"</html>"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		content := rootContent
		code := 200

		if r.URL.String() != "/" {
			code = 404
			content = NFContent
		}

		txBytes := len(content)
		fmt.Fprintln(w, content)
		logIt(r, txBytes, code)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		txBytes, err := GetSNGStats(w, socket)
		if err == nil {
			logIt(r, txBytes, 200)
		} else {
			log.Print(err)
		}
	})

	http.ListenAndServe(ip+":"+port, mux)
}
