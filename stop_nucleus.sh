#!/bin/bash
# stop_nucleus.sh — Force Stop NucleusMem cluster
# Run from project root: bash stop_nucleus.sh

PID_DIR="./pids"
BIN_DIR="./bin"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }

echo ""
echo -e "${YELLOW}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║        🛑 Stopping NucleusMem Cluster (Force Mode)     ║${NC}"
echo -e "${YELLOW}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================================
# Helper: Force Kill by Port
# ============================================================
force_kill_port() {
    local port=$1
    local name=$2
    # 查找占用端口的 PID
    local pid=$(lsof -t -i:$port 2>/dev/null)
    if [ -n "$pid" ]; then
        kill -9 $pid 2>/dev/null
        info "  Force killed $name on port $port (PID: $pid)"
    else
        # 静默成功，不报错
        true
    fi
}

# ============================================================
# Helper: Force Kill by Process Name Pattern
# ============================================================
force_kill_pattern() {
    local pattern=$1
    local name=$2
    if pkill -9 -f "$pattern" 2>/dev/null; then
        info "  Force killed processes matching '$pattern' ($name)"
    else
        true
    fi
}

# ============================================================
# 1. Kill Dynamically Launched Agents/Memspaces
# ============================================================
info "Stopping dynamic agents and memspaces..."
force_kill_pattern "bin/agent" "Dynamic Agents"
force_kill_pattern "bin/memspace" "Dynamic Memspaces"
sleep 1

# ============================================================
# 2. Stop Managed Services (By Port & PID)
# ============================================================
info "Stopping managed services..."

# 定义服务及其默认端口
declare -A SERVICES
SERVICES["chat-server"]=20001
SERVICES["embedding-server"]=20002
SERVICES["agent-manager"]=7007
SERVICES["agent-monitor"]=9091
SERVICES["memspace-manager"]=9200
SERVICES["memspace-monitor"]=9111

for name in "${!SERVICES[@]}"; do
    port=${SERVICES[$name]}
    pidfile="$PID_DIR/$name.pid"

    # 优先通过 PID 文件杀死
    if [ -f "$pidfile" ]; then
        pid=$(cat "$pidfile")
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null
            info "  Force killed $name (PID: $pid)"
        fi
        rm -f "$pidfile"
    fi

    # 兜底：如果 PID 文件失效，直接杀端口
    force_kill_port "$port" "$name"
done

# ============================================================
# 3. Stop TinyKV Cluster (Forcefully)
# ============================================================
info "Stopping TinyKV cluster..."
force_kill_pattern "pd-server" "TinyKV PD"
force_kill_pattern "tinykv-server" "TinyKV Store"

# 清理 TinyKV 数据以防止下次启动 WAL 错误
rm -rf default.pd*
rm -rf /tmp/kv1 /tmp/kv2 /tmp/kv3

# ============================================================
# 4. Cleanup Temp Files
# ============================================================
info "Cleaning temp files..."
rm -rf /tmp/nucleusmem/configs/*
rm -rf logs/*

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║        ✅ NucleusMem Cluster Force Stopped              ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"