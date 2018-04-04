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
	metrics     Metrics
	keyspace    string
	textfiledir string
)

func main() {

	metrics = NewMetrics()

	k := flag.String("keyspace", "", "Keyspace to repair.")
	lockprefix := flag.String("lockprefix", "", "Consul KV prefix.")
	tfdir := flag.String("textfiledir", "", "Prometheus node exporter textfile directory.")

	flag.Parse()

	keyspace = *k
	textfiledir = *tfdir

	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		fail("Could not instantiate Consul client: ", err)
	}

	metrics.lockstart = time.Now()
	lock, lockCh := acquireLock(client, *lockprefix)
	metrics.lockfinish = time.Now()

	metrics.repairstart = time.Now()
	repair(lockCh)
	metrics.repairfinish = time.Now()

	err = lock.Unlock()
	if err != nil {
		// This is not ideal, but not necessarily a problem because the repair succeeded.
		// The lock will be released automatically when the session expires.
		log.Print("Unable to unlock Consul lock: ", err)
	}

	metrics.finish = time.Now()
	metrics.success = 1
	writeMetrics()
}

// Only one node should be repaired at a time. All nodes compete
// for a lock until all of them eventually obtain it and get repaired.
func acquireLock(client *consul.Client, lockprefix string) (*consul.Lock, <-chan struct{}) {

	hostname, err := os.Hostname()
	if err != nil {
		fail("Could not get hostname: ", err)
	}

	s := client.Session()
	se := consul.SessionEntry{
		Name: lockprefix,
		TTL:  "5m",
	}
	sid, _, err := s.Create(&se, &consul.WriteOptions{})
	if err != nil {
		fail("Could not create Consul session: ", err)
	}

	o := consul.LockOptions{
		Key:     lockprefix,
		Session: sid,
		Value:   []byte(hostname),
	}

	lock, err := client.LockOpts(&o)
	if err != nil {
		fail("Could not instantiate Lock: ", err)
	}

	stopCh := make(chan struct{})
	lockCh, err := lock.Lock(stopCh)
	if err != nil {
		fail("Could not acquire lock: ", err)
	}

	return lock, lockCh
}

func repair(lockCh <-chan struct{}) {

	cmd := exec.Command(
		"nodetool",
		"repair",
		keyspace,
		"--full",
		"--dc-parallel",
		"--partitioner-range",
	)

	done := make(chan bool)

	go func(done chan bool) {
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			fail("Could not execute repair command: ", err)
		}
		log.Printf("%s\n", stdoutStderr)
		done <- true
	}(done)

	go func() {
		<-lockCh
		err := cmd.Process.Kill()
		if err != nil {
			log.Print("Failed to kill process: ", err)
		}
		fail("Session expired before repair completion: ", nil)
	}()

	<-done
}

func fail(str string, err error) {
	if err != nil {
		log.Print(str, err)
	}
	writeMetrics()
	log.Fatal("Repair failed.")
}

// Write metrics to file.
func writeMetrics() {

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
