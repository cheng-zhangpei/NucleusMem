#!/bin/bash

# Configuration
PD_SRC="github.com/pingcap-incubator/tinykv/scheduler/server"
KV_SRC="github.com/pingcap-incubator/tinykv/kv/main"
BIN_DIR="bin"

echo "[INFO] Starting TinyKV Cluster Deployment..."

# 1. Compile Binaries
mkdir -p $BIN_DIR

echo "[INFO] Compiling PD Server..."
go build -o $BIN_DIR/pd-server $PD_SRC
if [ $? -ne 0 ]; then
    echo "[ERROR] Failed to compile PD Server"
    exit 1
fi

echo "[INFO] Compiling TinyKV Server..."
go build -o $BIN_DIR/tinykv-server $KV_SRC
if [ $? -ne 0 ]; then
    echo "[ERROR] Failed to compile TinyKV Server"
    exit 1
fi

# 2. Clean up previous data
echo "[INFO] Cleaning up old data..."
rm -rf default.pd*
rm -rf /tmp/kv1 /tmp/kv2 /tmp/kv3
mkdir -p logs

# 3. Start PD (Scheduler)
echo "[INFO] Starting PD Server..."
$BIN_DIR/pd-server > logs/pd.log 2>&1 &
PD_PID=$!
echo "PD PID: $PD_PID"

sleep 3

# 4. Start Store 1
echo "[INFO] Starting Store 1 (Port 20160)..."
$BIN_DIR/tinykv-server -addr="127.0.0.1:20160" -path="/tmp/kv1" -scheduler="127.0.0.1:2379" -loglevel="debug" > logs/store1.log 2>&1 &
STORE1_PID=$!

# 5. Start Store 2
echo "[INFO] Starting Store 2 (Port 20161)..."
$BIN_DIR/tinykv-server -addr="127.0.0.1:20161" -path="/tmp/kv2" -scheduler="127.0.0.1:2379" -loglevel="debug" > logs/store2.log 2>&1 &
STORE2_PID=$!

# 6. Start Store 3
echo "[INFO] Starting Store 3 (Port 20162)..."
$BIN_DIR/tinykv-server -addr="127.0.0.1:20162" -path="/tmp/kv3" -scheduler="127.0.0.1:2379" -loglevel="debug" > logs/store3.log 2>&1 &
STORE3_PID=$!

echo "[SUCCESS] Cluster is running! Logs are in ./logs/"
echo "Press Ctrl+C to stop the cluster."

# Trap to kill processes on exit
trap "kill $PD_PID $STORE1_PID $STORE2_PID $STORE3_PID; exit" INT
wait