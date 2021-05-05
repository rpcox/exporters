package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

type MetricLine struct {   // Field names from 'syslog-ng-ctl stats' call
	objectType string  // SourceName
	id         string  // SourceId
	instance   string  // SourceInstance
	state      string  // State (a, d, o)
	statType   string  // Type (dropped, processed, ...)
	value      float64 // Number
}

func makeMetricName(MetricLine m) label string {
}


func parseLine(line string) (MetricLine, error) {
	chunk := strings.SplitN(strings.TrimSpace(line), ";", 6)

	num, err := strconv.ParseFloat(chunk[5], 64)
	if err != nil {
		return MetricLine{}, err
	}

	//if chunk[4] == "o" {  // an orphan, skip it
        //		
	//}

	return MetricLine{chunk[0], chunk[1], chunk[2], chunk[3], chunk[4], num}, nil
}

func getStats() {
	c, err := net.Dial("unix", "/var/lib/syslog-ng/syslog-ng.ctl")

	if err != nil {
		log.Print("syslog-ng.ctl connect fail:", err)
		return
	}

	defer c.Close()

	_, err = c.Write([]byte("STATS\n"))
	if err != nil {
		log.Print("syslog-ng.ctl write error:", err)
		return
	}

	buf := bufio.NewReader(c)

	_, err = buf.ReadString('\n')
	if err != nil {
		return
	}

	for {
		line, err := buf.ReadString('\n')

		if err != nil || line[0] == '.' {
			fmt.Println("** End of STATS **")
			break
		}

		data, err := parseLine(line)
		if err != nil {
			continue
		}

		fmt.Println(data)
	}

}

func main() {
	getStats()
}
