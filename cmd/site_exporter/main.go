package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const tool = "site_exporter"
const version = "0.1.0"

type FlagData struct {
	Bind        string
	LogFh       *os.File
	LogFileName string
	Port        string
	SiteList    string
}

func Initialize() FlagData {
	var ip string
	var port int
	var fd FlagData

	flag.StringVar(&fd.LogFileName, "log", "/var/log/site_exporter.log", "Logfile location")
	flag.StringVar(&ip, "ip", "0.0.0.0", "Server bind IP address")
	flag.IntVar(&port, "port", 9400, "Server bind port")
	flag.StringVar(&fd.SiteList, "site-list", "", "Location of site list file")
	flag.Parse()

	if fd.SiteList == "" {
		log.Fatal("-site-list required")
	}

	if _, err := os.Stat(fd.SiteList); err != nil {
		log.Fatal(err)
	}

	if tmp := net.ParseIP(ip); tmp == nil {
		log.Fatalf("invalid IP address: %s\n", ip)
	}
	fd.Bind = ip

	if !(port >= 0 && port <= 65535) {
		log.Fatalf("port out of range: %v\n", port)
	}
	fd.Port = strconv.Itoa(port)

	if err := StartLog(&fd); err != nil {
		log.Printf("logging to stderr: %v\n", err)
	}

	log.Println("bind: " + fd.Bind + ":" + fd.Port)

	return fd
}

func StartLog(fd *FlagData) error {
	if fd.LogFh != nil {
		fd.LogFh.Close()
		now := time.Now().Format(`20060102`)
		err := os.Rename(fd.LogFileName, fd.LogFileName+`-`+now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not move log file on reset: %v\n", err)
		}
		fd.LogFh = nil
	}

	fh, err := os.OpenFile(fd.LogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	log.SetOutput(fh)
	//	log.SetFlags(log.Lmicroseconds | log.LUTC | log.Ldate | log.Ltime | log.Lshortfile)
	log.SetFlags(log.Lmicroseconds | log.LUTC | log.Ldate | log.Ltime)
	log.Printf("%s v%s starting", tool, version)

	return nil
}

func sigHandler(sigChan chan os.Signal, server *http.Server, fd FlagData) {
	for sig := range sigChan {
		if sig == syscall.SIGUSR1 {
			log.Println("signal: resetting log file")
			log.Println(fd.LogFileName)
		} else if sig == syscall.SIGUSR2 {
			log.Println("signal: prep for site reload")
			siteReloadSignal = true
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
	fd := Initialize()

	ssc := NewSiteStatCollector(fd.SiteList)
	prometheus.Register(ssc)
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr: fd.Bind + ":" + fd.Port,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	go sigHandler(sigChan, server, fd)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
