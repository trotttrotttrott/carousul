package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Metrics struct {
	start   time.Time
	finish  time.Time
	success int
}

func NewMetrics() Metrics {
	return Metrics{start: time.Now(), success: 0}
}

var (
	metrics Metrics
)

func main() {

	metrics = NewMetrics()

	keyspace := flag.String("keyspace", "", "Keyspace to repair.")
	textfiledir := flag.String("textfiledir", "", "Prometheus node exporter textfile directory.")

	flag.Parse()

	repair(*keyspace)

	metrics.finish = time.Now()
	metrics.success = 1
	writeMetrics(*keyspace, *textfiledir)
}

func repair(keyspace string) {
	cmd := exec.Command(
		"nodetool",
		"repair",
		keyspace,
		"--full",
		"--sequential",
		"--partitioner-range",
	)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", stdoutStderr)
}

// Write metrics to file.
func writeMetrics(keyspace string, textfiledir string) {

	prefix := "cassandra_repair"

	var s []string

	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "success", keyspace, metrics.success))
	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "duration_total_milliseconds", keyspace, metrics.finish.Sub(metrics.start).Nanoseconds()/int64(time.Millisecond)))
	s = append(s, "")

	fp := fmt.Sprintf("%s/%s_%s.prom", textfiledir, prefix, keyspace)
	err := ioutil.WriteFile(fp, []byte(strings.Join(s, "\n")), 0644)
	if err != nil {
		log.Fatalln(os.Stderr, "failed to write metrics", fp, err)
	}
}
