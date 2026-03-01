#!/bin/bash

echo "[INFO] === RESTARTING CLUSTER ==="

# 1. Stop services
echo "[INFO] Stopping services..."
pkill -f pd-server
pkill -f tinykv-server
# Wait for processes to exit
sleep 2
# Kill stubborn processes
pkill -9 -f pd-server
pkill -9 -f tinykv-server

# 2. Clean up data (most important step!)
echo "[INFO] Cleaning up data directories..."
# Remove PD data directories (usually default.pd-xxx)
rm -rf default.pd*
# Remove TinyKV data directories (and lock files)
rm -rf /tmp/kv1
rm -rf /tmp/kv2
rm -rf /tmp/kv3

# 3. Restart cluster
echo "[INFO] Starting new cluster..."
./start-cluster.sh