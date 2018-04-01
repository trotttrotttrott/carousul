package main

import (
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"

	consul "github.com/hashicorp/consul/api"
)

func TestMain(m *testing.M) {
	for { // wait for Cassandra
		conn, _ := net.DialTimeout("tcp", "localhost:9042", time.Duration(10)*time.Second)
		if conn != nil {
			conn.Close()
			break
		}
	}
	now := time.Now()
	keyspace = fmt.Sprintf("testkeyspace%02d%02d", now.Minute(), now.Second())
	createSeedData()
	metrics = NewMetrics()
	m.Run()
	destroySeedData()
}

func TestObtainLock(t *testing.T) {

	client, err := consul.NewClient(&consul.Config{Address: "consul:8500"})
	if err != nil {
		panic(err)
	}

	lock, _ := acquireLock(client, "dc1", "some-prefix")

	err = lock.Unlock()
	if err != nil {
		panic(err)
	}
}

func TestRepair(t *testing.T) {
	lockCh := make(chan struct{})
	repair(lockCh)
}

func TestWriteMetrics(t *testing.T) {
	metrics.start = time.Now()
	metrics.lockstart = metrics.start.Add(time.Second + 1)
	metrics.lockfinish = metrics.lockstart.Add(time.Second + 5)
	metrics.repairstart = metrics.lockfinish.Add(time.Second + 1)
	metrics.repairfinish = metrics.repairstart.Add(time.Second + 10)
	metrics.finish = metrics.repairfinish.Add(time.Second + 1)
	metrics.success = 1
	writeMetrics()
}

// setup/teardown

func createSeedData() {

	var cmds []string

	cmds = append(cmds, fmt.Sprintf("create keyspace %s with replication = { 'class' : 'SimpleStrategy', 'replication_factor' : 1 };", keyspace))
	cmds = append(cmds, fmt.Sprintf("create table %s.account(id UUID, email UUID, first_name UUID, last_name UUID, PRIMARY KEY(id));", keyspace))
	cmds = append(cmds, fmt.Sprintf("create table %s.organization(id UUID, identifier UUID, PRIMARY KEY(id));", keyspace))

	for i := 0; i <= 5; i++ {
		cmds = append(cmds, fmt.Sprintf("insert into %s.account (id, email, first_name, last_name) values (uuid(), uuid(), uuid(), uuid());", keyspace))
		cmds = append(cmds, fmt.Sprintf("insert into %s.organization (id, identifier) values (uuid(), uuid());", keyspace))
	}

	for _, cmd := range cmds {
		_, err := exec.Command("cqlsh", "-e", cmd).CombinedOutput()
		if err != nil {
			panic(err)
		}
	}
}

func destroySeedData() {
	cmd := exec.Command("cqlsh", "-e", fmt.Sprintf("drop keyspace %s", keyspace))
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
}
