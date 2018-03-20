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

	consul "github.com/hashicorp/consul/api"
)

type Metrics struct {

	// time how long it takes to acquire the lock.
	lockstart  time.Time
	lockfinish time.Time

	// time how long repair takes.
	repairstart  time.Time
	repairfinish time.Time

	// time entire process.
	start  time.Time
	finish time.Time

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
	lockdc := flag.String("lockdc", "", "Datacenter where the Consul lock will be located.")
	lockprefix := flag.String("lockprefix", "", "Consul KV prefix.")
	lockname := flag.String("lockname", "", "Lock name.")
	textfiledir := flag.String("textfiledir", "", "Prometheus node exporter textfile directory.")

	flag.Parse()

	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	metrics.lockstart = time.Now()
	lock, lockCh := obtainLock(client, *lockdc, *lockprefix, *lockname)
	metrics.lockfinish = time.Now()

	metrics.repairstart = time.Now()
	repair(*keyspace, lockCh)
	metrics.repairfinish = time.Now()

	err = lock.Unlock()
	if err != nil {
		// This is not ideal, but not necessarily a problem because the repair succeeded.
		// The lock will be released automatically when the session expires which will be in about 30s.
		log.Print("Unable to unlock Consul lock: ", err)
	}

	metrics.finish = time.Now()
	metrics.success = 1
	writeMetrics(*keyspace, *textfiledir)
}

// Only one node should be repaired at a time. All nodes compete
// for a lock until all of them eventually obtain it and get repaired.
func obtainLock(client *consul.Client, lockdc string, lockprefix string, lockname string) (*consul.Lock, <-chan struct{}) {

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	s := client.Session()
	se := consul.SessionEntry{
		Name: lockprefix,
	}
	q := consul.WriteOptions{
		Datacenter: lockdc,
	}
	sid, _, err := s.Create(&se, &q)
	if err != nil {
		log.Fatal(err)
	}

	o := consul.LockOptions{
		Key:     lockname,
		Session: sid,
		Value:   []byte(hostname),
	}

	lock, err := client.LockOpts(&o)
	if err != nil {
		log.Fatal(err)
	}

	stopCh := make(chan struct{})
	lockCh, err := lock.Lock(stopCh)
	if err != nil {
		log.Fatal(err)
	}

	return lock, lockCh
}

func repair(keyspace string, lockCh <-chan struct{}) {

	cmd := exec.Command(
		"nodetool",
		"repair",
		keyspace,
		"--full",
		"--sequential",
		"--partitioner-range",
	)

	go func() {
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", stdoutStderr)
	}()

	go func() {
		<-lockCh
		err := cmd.Process.Kill()
		if err != nil {
			log.Fatal("Failed to kill process: ", err)
		}
	}()
}

// Write metrics to file.
func writeMetrics(keyspace string, textfiledir string) {

	prefix := "cassandra_repair"

	var s []string

	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "success", keyspace, metrics.success))
	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "duration_lock_milliseconds", keyspace, metrics.lockfinish.Sub(metrics.lockstart).Nanoseconds()/int64(time.Millisecond)))
	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "duration_repair_milliseconds", keyspace, metrics.repairfinish.Sub(metrics.repairstart).Nanoseconds()/int64(time.Millisecond)))
	s = append(s, fmt.Sprintf("%s_%s{keyspace=\"%s\"} %v", prefix, "duration_total_milliseconds", keyspace, metrics.finish.Sub(metrics.start).Nanoseconds()/int64(time.Millisecond)))
	s = append(s, "")

	fp := fmt.Sprintf("%s/%s_%s.prom", textfiledir, prefix, keyspace)
	err := ioutil.WriteFile(fp, []byte(strings.Join(s, "\n")), 0644)
	if err != nil {
		log.Fatalln(os.Stderr, "failed to write metrics", fp, err)
	}
}
