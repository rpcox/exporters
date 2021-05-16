package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

type MetricType struct {
  counter string
  gauge   string
}


type SNG_MetricLine struct {   // Field names from 'syslog-ng-ctl stats' call
	objectType string  // SourceName
	id         string  // SourceId
	instance   string  // SourceInstance
	state      string  // State (a, d, o)
	statType   string  // Type (dropped, processed, ...)
	value      float64 // Number
}

func TypeLine (metricName string, metricType string) {
	s:= []string{"# TYPE ", metricName, " ", metricType}
	return strings.Join(s,"_")
}

func MetricLine(metricName string, SNG_MetricLine ml) {
	s:= []string{metricName, "{id\"", ml.id, "\",item=\"", ml.item, "\",state=\"", ml.state, "\",type=\"", ml.statType, "\"} "}
}

func MetricName(MetricLine m) string {
	s:= []string{"sng", m.objectType}
	s = strings.Join(s, "_")
	return strings.ReplaceAll(s, ".", "_")
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

func GetSNGStats() {
	mt := MetricType("counter", "gauge")
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

	for {
		line, err := buf.ReadString('\n')

		if err != nil || line[0] == '.' {
			fmt.Println("** End of STATS **")
			break
		}

		if line[4] == 'o' || line[4] == 'd' { // don't want orphans or dynamics
			continue
		}

		sngData, err := parseLine(line)
		if err != nil {
			fmt.Println("parse error: ", err)
			continue
		}

		name := generateMetricName(line)

		switch sngData.objectType[0:4] {
		case "src.":
			        switch sngData.statType {
				case "processed":
				        fmt.Println(TypeLine(name, mt.gauge))
				case "stamp":
				        fmt.Println(TypeLine(name, mt.counter)
				}
			case "dst.":
			case "filt":
			//default:
		}

		fmt.Println(MetricLine(name, sngData)))
	}

}


func main() {
	GetSNGStats()
}
