### sng-export

Export Syslog-NG metrics for Prometheus import.

#### build

    git clone https://github.com/rpcox/sng_export.git
    cd sng_export
    go build sng-export.go

#### install

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


