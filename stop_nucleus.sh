#!/bin/bash
# stop_nucleus.sh — Stop NucleusMem cluster
# Run from project root: bash stop_nucleus.sh

set -e

PID_DIR="./pids"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }

echo ""
echo -e "${YELLOW}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║        🛑 Stopping NucleusMem Cluster                  ║${NC}"
echo -e "${YELLOW}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================================
# Kill dynamically launched agents and memspaces
# ============================================================
info "Stopping dynamically launched processes..."
pkill -f "bin/agent --config /tmp/nucleusmem" 2>/dev/null && info "  Killed dynamic agents" || true
pkill -f "bin/memspace --config /tmp/nucleusmem" 2>/dev/null && info "  Killed dynamic memspaces" || true
sleep 1

# ============================================================
# Stop managed services (reverse order)
# ============================================================
stop_service() {
    local name=$1
    local pidfile="$PID_DIR/$name.pid"

    if [ -f "$pidfile" ]; then
        local pid=$(cat "$pidfile")
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid"
            info "  Stopped $name (PID: $pid)"
        else
            warn "  $name (PID: $pid) already stopped"
        fi
        rm -f "$pidfile"
    else
        warn "  No PID file for $name"
    fi
}

stop_service "agent-monitor"
stop_service "agent-manager"
stop_service "memspace-monitor"
stop_service "memspace-manager"
stop_service "embedding-server"
stop_service "chat-server"

# ============================================================
# Stop TinyKV
# ============================================================
info "Stopping TinyKV cluster..."
if [ -f "./stop_cluster.sh" ]; then
    bash ./stop_cluster.sh
    info "  TinyKV cluster stopped"
else
    warn "  stop_cluster.sh not found, skipping"
fi

# ============================================================
# Cleanup
# ============================================================
info "Cleaning temp files..."
rm -rf /tmp/nucleusmem/configs/*

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║        ✅ NucleusMem Cluster Stopped                    ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"