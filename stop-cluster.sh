#!/bin/bash

echo "[INFO] Stopping TinyKV Cluster..."

# 1. 查找并杀死 pd-server
echo "[INFO] Killing PD Server..."
pkill -f pd-server

# 2. 查找并杀死 tinykv-server
echo "[INFO] Killing TinyKV Servers..."
pkill -f tinykv-server

# 3. 再次确认是否杀干净了 (强杀)
sleep 1
pkill -9 -f pd-server
pkill -9 -f tinykv-server

echo "[SUCCESS] Cluster stopped."