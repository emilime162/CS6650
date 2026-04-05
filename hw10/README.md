# CS6650 Assignment 4 – Distributed KV Store

## Project Structure

```
cs6650-a4/
├── cmd/node/main.go             # single binary, all node types
├── internal/
│   ├── store/store.go           # versioned in-memory KV store
│   ├── leader/handler.go        # leader + follower HTTP handlers
│   └── leaderless/handler.go    # leaderless node handler
├── tests/consistency_test.go    # black-box consistency + window tests
├── loadtest/
│   ├── main.go                  # load test client
│   └── plot.py                  # graph generator for report
└── docker/
    ├── Dockerfile
    ├── compose-lf-w5r1.yml      # Leader-Follower W=5 R=1
    ├── compose-lf-w1r5.yml      # Leader-Follower W=1 R=5
    ├── compose-lf-quorum.yml    # Leader-Follower W=3 R=3
    └── compose-leaderless.yml   # Leaderless W=5 R=1
```

---

## Running the Clusters

### Leader-Follower W=5 R=1
```bash
docker compose -f docker/compose-lf-w5r1.yml up --build
```
- Leader on port 8080, followers on 8081-8084.
- All writes go to `localhost:8080`.

### Leader-Follower W=1 R=5
```bash
docker compose -f docker/compose-lf-w1r5.yml up --build
```

### Leader-Follower Quorum W=3 R=3
```bash
docker compose -f docker/compose-lf-quorum.yml up --build
```

### Leaderless W=5 R=1
```bash
docker compose -f docker/compose-leaderless.yml up --build
```
- All 5 nodes on ports 9080-9084.
- Reads and writes can go to any port.

---

## Running Consistency Tests

```bash
# Start the cluster you want to test, then:
go test ./tests/... -v -run TestLeader       # leader-follower tests
go test ./tests/... -v -run TestLeaderless   # leaderless tests
```

---

## Running the Load Tester

```bash
# Leader-Follower, 10% writes, 2000 requests
go run loadtest/main.go \
  -leader   http://localhost:8080 \
  -followers http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  -requests 2000 \
  -concurrency 20 \
  -write-pct 10 \
  -key-pool 20 \
  -out results_lf_w5r1_10w.csv

# Leaderless
go run loadtest/main.go \
  -leaderless \
  -leader   http://localhost:9080 \
  -followers http://localhost:9081,http://localhost:9082,http://localhost:9083,http://localhost:9084 \
  -requests 2000 \
  -concurrency 20 \
  -write-pct 10 \
  -key-pool 20 \
  -out results_leaderless_10w.csv
```

### All 16 load test runs (4 configs × 4 ratios)

| Config        | Write % | Read % | Output file                         |
|---------------|---------|--------|-------------------------------------|
| LF W=5 R=1    | 1       | 99     | results_lf_w5r1_1w.csv             |
| LF W=5 R=1    | 10      | 90     | results_lf_w5r1_10w.csv            |
| LF W=5 R=1    | 50      | 50     | results_lf_w5r1_50w.csv            |
| LF W=5 R=1    | 90      | 10     | results_lf_w5r1_90w.csv            |
| LF W=1 R=5    | 1       | 99     | results_lf_w1r5_1w.csv             |
| ...           | ...     | ...    | ...                                 |
| Leaderless    | 90      | 10     | results_leaderless_90w.csv          |

---

## Generating Report Graphs

```bash
pip install matplotlib numpy
python3 loadtest/plot.py \
  --files results_lf_w5r1_10w.csv results_lf_w1r5_10w.csv results_lf_quorum_10w.csv results_leaderless_10w.csv \
  --labels "W5R1 10%w" "W1R5 10%w" "Quorum 10%w" "Leaderless 10%w" \
  --out-dir report_graphs
```

Produces:
- `latency_cdf.png` – CDF of read and write latency (shows long tail)
- `avg_latency.png` – grouped bar chart
- `stale_reads.png` – stale read % per config

---

## Key Design Decisions

### Version numbers
Every KV entry carries a monotonically increasing `Version int64`. The leader
assigns the version on write. Followers accept the incoming version during
replication. This is how R=5 / R=3 reads pick the freshest value from a pool
of responses.

### Simulated delays
| Event                          | Delay    |
|-------------------------------|----------|
| Leader after each follower msg | 200 ms   |
| Follower on receiving write    | 100 ms   |
| Follower on internal read      |  50 ms   |

### W=5 R=1 write time
With 4 followers and 200 ms between each: minimum write latency ≈ 800 ms.
Set your load tester timeout to at least 5 s.

### Key pool clustering
The load tester uses a small key pool (default 20 keys). With 20 concurrent
workers hammering 20 keys, reads and writes to the same key cluster naturally
within milliseconds of each other, maximising stale read exposure.
