# Carousul

**C**assandra **A**nti-entropy **R**epair **ou** Con**sul**

Program for performing [anti-entropy repair](https://docs.datastax.com/en/cassandra/latest/cassandra/operations/opsRepairNodesManualRepair.html) with the [`nodetool repair`](https://docs.datastax.com/en/cassandra/latest/cassandra/tools/toolsRepair.html) command and [Consul](https://www.consul.io/) for distributed locking.

## Flags

**keyspace**: Cassandra keyspace to repair.

**lockprefix**: Consul KV prefix indicating where locks are to be created.

**textfiledir**: Prometheus node exporter textfile directory. A file in [text-based exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/#text-based-format) will be written for collection.

## Repair Considerations

### Full vs. Incremental

Only full repairs are done.

There is some conflicting recommendations on this topic. Using incremental repairs are compelling because they reduce repair time significantly. They were claimed to be ["more efficient"](https://www.datastax.com/dev/blog/more-efficient-repairs) when they became available in Cassandra 2.1. However, they do not maintain data integrity. It is also stated to be "not recommended" in the [repair command docs](https://docs.datastax.com/en/cassandra/latest/cassandra/tools/toolsRepair.html#toolsRepair__incremental). Therefore, only full repairs are done.

### Partitioner Range

Repairs only the primary partition ranges of the node being repaired. This prevents Cassandra from repairing the same range of data several times. It is also the recommended approach for routine maintenance.

### Parallel vs. DC-Parallel vs. Sequential

DC-Parallel repair is used.

Sequential repair requires that each node of a cluster run a repair command one after the other. This repair strategy entails maximum operational overhead.

Parallel repair repairs all nodes in all datacenters at the same time. This repair strategy entails maximum performance impact.

DC-Parallel combines them by running sequential repairs across datacenters in parallel. This means that a complete repair can be accomplished by running repairs on each node of just a single datacenter one at a time.

Compared to sequential repair, this is less operationally complex. It is much easier to automate dc-parallel repair in a single datacenter as opposed to sequential repair across all datacenters. Especially since only one node in the entire cluster can be repaired at a time.

Compared to parallel, this is less resource intensive. This is because only nodes that own replica data in common with the coordinator node's primary partition range will be doing work.

## Distributed Locking

Distributed locking is implemented in order to ensure that this program is executed one node at a time. The program is to be run simultaneously on the nodes of a "coordinating" datacenter. The coordinating datacenter is the only datacenter that needs to run repair because "dc-parallel" repair is implemented. The nodes will proceed to compete for a lock. Repair will not happen until the lock is obtained and each node will eventually obtain the lock. When all nodes of the coordinating datacenter get a turn, the targeted keyspace will have been fully repaired.

Consul is used to achieve this. [Consul sessions](https://www.consul.io/docs/internals/sessions.html) are the basis for the approach. Sessions address the following concerns:

### Lock Release on Failure

Sessions have a TTL. A lock will be released when its session expires.

### Repair Exceeds TTL of Session

As long as the program is alive, its session will be automatically renewed. This is part of [`func (*Lock) Lock`](https://godoc.org/github.com/hashicorp/consul/api#Lock.Lock) in Consul's API client.

### Unexpected Session Expiration

If a session is invalidated before a repair is completed, the repair will be interrupted. While the interruption results in a failed repair, the rest of the cluster will be able to continue safely.
