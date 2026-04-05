#!/bin/bash
# run_all_loadtests.sh
# Runs all 16 load test combinations (4 configs × 4 write ratios).
# Run this from the root of the cs6650-a4 project directory.
# Results are saved as CSV files in the current directory.

set -e  # exit on any error

REQUESTS=500
CONCURRENCY=10
KEY_POOL=20

echo "======================================================"
echo " CS6650 A4 – Full Load Test Run"
echo " requests=$REQUESTS  concurrency=$CONCURRENCY  keys=$KEY_POOL"
echo "======================================================"

# ── Helper: wait until all ports are ready ─────────────────────────────────
wait_ready() {
  local ports=("$@")
  for port in "${ports[@]}"; do
    echo -n "  Waiting for port $port..."
    until curl -sf "http://localhost:$port/health" > /dev/null 2>&1; do
      sleep 0.5
    done
    echo " ready"
  done
}

# ── 1. Leader-Follower  W=5 R=1 ────────────────────────────────────────────
echo ""
echo ">>> Starting W=5 R=1 cluster..."
docker compose -f docker/compose-lf-w5r1.yml up --build -d
wait_ready 8080 8081 8082 8083 8084

for pct in 1 10 50 90; do
  echo "  Running W=5 R=1  write-pct=$pct%..."
  go run loadtest/main.go \
    -leader      http://localhost:8080 \
    -followers   http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
    -requests    $REQUESTS \
    -concurrency $CONCURRENCY \
    -write-pct   $pct \
    -key-pool    $KEY_POOL \
    -out         results_lf_w5r1_${pct}w.csv
done

docker compose -f docker/compose-lf-w5r1.yml down
echo ">>> W=5 R=1 done."

# ── 2. Leader-Follower  W=1 R=5 ────────────────────────────────────────────
echo ""
echo ">>> Starting W=1 R=5 cluster..."
docker compose -f docker/compose-lf-w1r5.yml up --build -d
wait_ready 8080 8081 8082 8083 8084

for pct in 1 10 50 90; do
  echo "  Running W=1 R=5  write-pct=$pct%..."
  go run loadtest/main.go \
    -leader      http://localhost:8080 \
    -followers   http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
    -requests    $REQUESTS \
    -concurrency $CONCURRENCY \
    -write-pct   $pct \
    -key-pool    $KEY_POOL \
    -out         results_lf_w1r5_${pct}w.csv
done

docker compose -f docker/compose-lf-w1r5.yml down
echo ">>> W=1 R=5 done."

# ── 3. Leader-Follower  Quorum W=3 R=3 ─────────────────────────────────────
echo ""
echo ">>> Starting Quorum W=3 R=3 cluster..."
docker compose -f docker/compose-lf-quorum.yml up --build -d
wait_ready 8080 8081 8082 8083 8084

for pct in 1 10 50 90; do
  echo "  Running Quorum  write-pct=$pct%..."
  go run loadtest/main.go \
    -leader      http://localhost:8080 \
    -followers   http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
    -requests    $REQUESTS \
    -concurrency $CONCURRENCY \
    -write-pct   $pct \
    -key-pool    $KEY_POOL \
    -out         results_lf_quorum_${pct}w.csv
done

docker compose -f docker/compose-lf-quorum.yml down
echo ">>> Quorum done."

# ── 4. Leaderless  W=5 R=1 ─────────────────────────────────────────────────
echo ""
echo ">>> Starting Leaderless cluster..."
docker compose -f docker/compose-leaderless.yml up --build -d
wait_ready 9080 9081 9082 9083 9084

for pct in 1 10 50 90; do
  echo "  Running Leaderless  write-pct=$pct%..."
  go run loadtest/main.go -leaderless \
    -leader      http://localhost:9080 \
    -followers   http://localhost:9081,http://localhost:9082,http://localhost:9083,http://localhost:9084 \
    -requests    $REQUESTS \
    -concurrency $CONCURRENCY \
    -write-pct   $pct \
    -key-pool    $KEY_POOL \
    -out         results_leaderless_${pct}w.csv
done

docker compose -f docker/compose-leaderless.yml down
echo ">>> Leaderless done."

# ── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo "======================================================"
echo " All 16 runs complete. CSV files:"
ls -1 results_*.csv
echo ""
echo " Next: generate graphs with:"
echo "   pip install matplotlib numpy"
echo "   python3 loadtest/plot.py \\"
echo "     --files results_lf_w5r1_1w.csv results_lf_w5r1_10w.csv results_lf_w5r1_50w.csv results_lf_w5r1_90w.csv \\"
echo "             results_lf_w1r5_1w.csv results_lf_w1r5_10w.csv results_lf_w1r5_50w.csv results_lf_w1r5_90w.csv \\"
echo "             results_lf_quorum_1w.csv results_lf_quorum_10w.csv results_lf_quorum_50w.csv results_lf_quorum_90w.csv \\"
echo "             results_leaderless_1w.csv results_leaderless_10w.csv results_leaderless_50w.csv results_leaderless_90w.csv \\"
echo "     --labels 'W5R1 1%w' 'W5R1 10%w' 'W5R1 50%w' 'W5R1 90%w' \\"
echo "              'W1R5 1%w' 'W1R5 10%w' 'W1R5 50%w' 'W1R5 90%w' \\"
echo "              'Quorum 1%w' 'Quorum 10%w' 'Quorum 50%w' 'Quorum 90%w' \\"
echo "              'LL 1%w' 'LL 10%w' 'LL 50%w' 'LL 90%w' \\"
echo "     --out-dir report_graphs"
echo "======================================================"