### sng-export

Export Syslog-NG metrics for Prometheus import.

#### build

    git clone https://github.com/rpcox/sng_export.git
    cd sng_export
    go build sng-export.go

#### install

    In syslog-ng.conf, set "stats-level(1)".  Then, 

    mkdir /opt/sng-export
    cp sng-export /opt/sng-export
    cp sng-export.service /etc/systemd/system
    systemctl daemon-reload
    systemctl enable sng-export
    systemctl start sng-export

#### usage

    # /opt/sng-export/sng-export -h
    Usage of /opt/sng-export/sng-export:
      -ip string
            Server bind IP address (default "0.0.0.0")
      -log-path string
            Logfile location (default "/var/log/sng-export.log")
      -port string
            Server bind port (default "8000")
      -socket-path string
           syslog-ng.ctl socket location (default "/var/lib/syslog-ng/syslog-ng.ctl")

#### monitor

sng-export creates a log at /var/log/sng-export.log by default (see usage)

format: date time.ms prometheus_server_ip "method url" bytes_sent http_status "referer" user-agent

    # tail -f /var/log/sng-export.log 
    2021/08/18 02:52:38.378423 10.11.12.13 "GET /metrics" 8739 200 "-" Prometheus/2.26.0
    2021/08/18 02:52:53.379103 10.11.12.13 "GET /metrics" 8903 200 "-" Prometheus/2.26.0
    2021/08/18 02:53:08.377324 10.11.12.13 "GET /metrics" 8283 200 "-" Prometheus/2.26.0
    2021/08/18 02:53:23.378199 10.11.12.13 "GET /metrics" 8903 200 "-" Prometheus/2.26.0
    2021/08/18 02:53:38.377649 10.11.12.13 "GET /metrics" 7475 200 "-" Prometheus/2.26.0
    2021/08/18 02:53:53.379115 10.11.12.13 "GET /metrics" 6715 200 "-" Prometheus/2.26.0
    2021/08/18 02:54:08.379129 10.11.12.13 "GET /metrics" 6715 200 "-" Prometheus/2.26.0
    2021/08/18 02:54:23.377992 10.11.12.13 "GET /metrics" 7474 200 "-" Prometheus/2.26.0
    2021/08/18 02:54:38.378396 10.11.12.13 "GET /metrics" 7474 200 "-" Prometheus/2.26.0
    2021/08/18 02:54:53.378353 10.11.12.13 "GET /metrics" 7474 200 "-" Prometheus/2.26.0

#### test

    # curl localhost:8000/
    <html>
     <head><title>Syslog-NG Exporter</title></head>
      <body>
      <h1>Syslog-NG Exporter</h1>
      <p><a href="/metrics">Metrics</a></p>
    </body>
    </html>

    # curl localhost:8000/metrics
    ....

