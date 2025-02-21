// Syslog-NG exporter for Prometheus
// Spews raw default csv stats or the Syslog-NG PE Prometheus stats
package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const tool = "sng_exporter_raw"
const version = "0.0.1"

type CtlData struct {
	Bind        string
	LogFh       *os.File
	LogFileName string
	Port        string
	Socket      string
	Prometheus  bool
}

func Initialize() CtlData {
	var ip string
	var port int
	var ctl CtlData

	flag.StringVar(&ctl.LogFileName, "log-path", "/var/log/sng-export.log", "Logfile location")
	flag.StringVar(&ctl.Socket, "socket-path", "/var/lib/syslog-ng/syslog-ng.ctl", "Syslog-NG domain socket location")
	flag.BoolVar(&ctl.Prometheus, "prom", false, "Prometheus or default stats")
	flag.StringVar(&ip, "ip", "0.0.0.0", "Server bind IP address")
	flag.IntVar(&port, "port", 9500, "Server bind port")

	tmp := net.ParseIP(ip)
	if tmp == nil {
		log.Fatalf("invalid IP address: %s\n", ip)
	}
	ctl.Bind = ip

	if !(port >= 0 && port <= 65535) {
		log.Fatalf("port out of range: %v\n", port)
	}
	ctl.Port = strconv.Itoa(port)

	err := StartLog(&ctl)
	if err != nil {
		log.Printf("not logging to a file: %v\n", err)
	}

	log.Println("bind: " + ctl.Bind + ":" + ctl.Port)
	log.Println("syslog-ng socket: " + ctl.Socket)

	return ctl
}

func StartLog(ctl *CtlData) error {
	if ctl.LogFh != nil {
		ctl.LogFh.Close()
		ctl.LogFh = nil
	}

	fh, err := os.OpenFile(ctl.LogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err) // using stderr
		return err
	}

	log.SetOutput(fh)
	log.SetFlags(log.Lmicroseconds | log.LUTC | log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("%s starting", tool)

	return nil
}

var rootContent = `<html>
 <head><title>Syslog-NG Exporter</title></head> 
<body><h1>Syslog-NG Exporter</h1> <p><a href="/metrics">Metrics</a></p></body>
 </html>`

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.Write([]byte(rootContent))
	log.Printf(r.RemoteAddr + " \"" + r.Method + " " + r.URL.String() + "\"+" + r.Header.Get("User-Agent"))
}

var NFContent = `<html><head><title>Syslog-NG Exporter</title></head>
<body><h1>404 Not Found</h1></body>
</html>`

func notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	log.Printf(r.RemoteAddr + " \"" + r.Method + " " + r.URL.String() + "\"+" + r.Header.Get("User-Agent"))
}

func notAllowed(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	log.Printf(r.RemoteAddr + " \"" + r.Method + " " + r.URL.String() + "\"+" + r.Header.Get("User-Agent"))
}

func GetRawMetrics(w http.ResponseWriter, socket string, prom bool) (int, error) {
	query := "STATS"
	if prom {
		query = "STATS PROMETHEUS"
	}

	c, err := net.Dial("unix", socket)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return 0, err
	}
	defer c.Close()

	_, err = c.Write([]byte(query))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("query=%s: %s\n", query, err)
		return 0, err
	}

	r := bufio.NewReader(c)
	content, err := io.ReadAll(r)
	if err != nil {
		log.Print(err)
		return 0, err
	}

	w.Write(content)

	return len(content), nil
}

func sigHandler(sigChan chan os.Signal, server *http.Server, ctl CtlData) {
	for sig := range sigChan {
		if sig == syscall.SIGUSR1 {
			log.Println("signal: resetting log file")
			log.Println(ctl.LogFileName)
		} else if sig == syscall.SIGTERM || sig == syscall.SIGINT {
			log.Println("signal: shutting down")
			ctx, shutdownRelease := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownRelease()

			if err := server.Shutdown(ctx); err != nil {
				log.Fatalf("HTTP shutdown error: %v", err)
			}
			return
		}
	}
}

func main() {
	flag.Parse()
	ctl := Initialize()
	r := mux.NewRouter()

	metricsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		n, err := GetRawMetrics(w, ctl.Socket, ctl.Prometheus)
		if err == nil {
			log.Println(r.RemoteAddr+" \""+r.Method+" "+r.URL.String()+"\"+"+r.Header.Get("User-Agent"), 200, n)
		} else {
			log.Print(err)
		}
	}

	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/metrics", metricsHandler).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(notAllowed)

	server := &http.Server{
		Addr:    ctl.Bind + ":" + ctl.Port,
		Handler: r,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	go sigHandler(sigChan, server, ctl)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
