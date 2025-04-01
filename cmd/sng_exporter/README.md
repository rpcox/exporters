### sng_exporter

Export Syslog-NG (SNG) metrics for Prometheus import. sng_exporter was originally written to run on a P1 to solve an network issue at my home. Syslog-NG was not exporting Prometheus metrics at that time, but if I used the Prometheus client libraries, the client was too fat to run well on the P1 (timeouts, crashes, ...), so I resorted to brute forcing the metric format so I could get the data into Prometheus and figure out the issue plaguing my home network.

As soon as I get some time, this one will get a big upgrade.