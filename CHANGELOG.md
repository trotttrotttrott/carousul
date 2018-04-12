# 0.0.3 (2018-04-11)

* Print output of `nodetool repair` before failing if an error exists.
* `func fail` logs arguments regardless if the error is nil.

# 0.0.2 (2018-04-09)

* Do not write duration metrics without a set finish value.
* Metric names have keyspace segment. Node exporter does not support having the same metric name in many files.

# 0.0.1 (2018-03-16)

* Initial release.
