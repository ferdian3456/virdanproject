#!/bin/bash

# Script untuk run multiple k6 containers secara parallel
# Setiap k6 container generate 10k RPS

TOTAL_K6_CONTAINERS=5  # Total k6 instances yang akan run parallel
RPS_PER_INSTANCE=10000  # 10k RPS per k6 instance

echo "Starting load test dengan $TOTAL_K6_CONTAINERS k6 containers..."
echo "Target RPS: $((TOTAL_K6_CONTAINERS * RPS_PER_INSTANCE)) RPS"
echo ""

# Run multiple k6 instances di background
for i in $(seq 1 $TOTAL_K6_CONTAINERS); do
  echo "Starting k6 instance #$i..."
  k6 run --no-usage-report k6_10k_test.js > k6_result_$i.log 2>&1 &
done

echo ""
echo "All k6 instances started!"
echo "Check individual results: k6_result_*.log"
echo ""
echo "Wait untuk semua processes selesai..."
wait

echo ""
echo "=========================================="
echo "LOAD TEST COMPLETE!"
echo "=========================================="
echo ""

# Aggregate results
echo "=== AGGREGATE RESULTS ==="
total_rps=0
total_reqs=0

for i in $(seq 1 $TOTAL_K6_CONTAINERS); do
  if [ -f "k6_result_$i.log" ]; then
    rps=$(grep "http_reqs......................" k6_result_$i.log | awk '{print $2}')
    echo "Instance #$i: $rps RPS"

    # Extract numeric value (remove /s)
    rps_num=$(echo $rps | sed 's#/s##')
    total_rps=$(echo "$total_rps + $rps_num" | bc)
  fi
done

echo ""
echo "Total RPS across all instances: $total_rps RPS"
echo ""
echo "Detailed results: k6_result_*.log"
