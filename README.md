# Repair

Program for performing [anti-entropy repair](https://docs.datastax.com/en/cassandra/latest/cassandra/operations/opsRepairNodesManualRepair.html) with the [`nodetool repair`](https://docs.datastax.com/en/cassandra/latest/cassandra/tools/toolsRepair.html) command.

## Flags

**keyspace**: Keyspace to repair.

**lockdc**: Consul datacenter where the lock should live.

**lockprefix**: Consul KV prefix.

**lockname**: Lock name.

**textfiledir**: Prometheus node exporter textfile directory.

## Considerations

### Full vs. Incremental

Only full repairs are done.

There is some conflicting recommendations on this topic. Using incremental repairs are compelling because they reduce repair time significantly. They were claimed to be ["more efficient"](https://www.datastax.com/dev/blog/more-efficient-repairs) when they became available in Cassandra 2.1. However, they do not maintain data integrity. It is also stated to be "not recommended" in the [repair command docs](https://docs.datastax.com/en/cassandra/latest/cassandra/tools/toolsRepair.html#toolsRepair__incremental). Therefore, only full repairs are done.

### Sequential vs. Parallel

Sequential repair is used instead of parallel. While parallel is faster, it is much more resource intensive and can be expected to hurt cluster performance.

### Partitioner Range

Repairs only the primary partition ranges of the node being repaired. This prevents Cassandra from repairing the same range of data several times. It is also the recommended approach for routine maintenance.

### Cluster-Wide

Cluster-wide repair increases network traffic between datacenters tremendously, and can cause cluster issues. However, it is required for performing primary range repair (partitioner range).
