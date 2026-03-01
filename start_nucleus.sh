#!/bin/bash
# start_nucleus.sh — Start NucleusMem cluster
# Run from project root: bash start_nucleus.sh
# 看当前目录
pwd
# 看文件在不在
ls -la start_cluster.sh
set -e

BIN_DIR="./bin"
CONFIG_DIR="./pkg/configs/file"
LOG_DIR="./logs"
PID_DIR="./pids"

mkdir -p "$LOG_DIR" "$PID_DIR"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }
step()  { echo -e "${BLUE}[STEP]${NC}  $1"; }

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║        🌳 NucleusMem Cluster Launcher                  ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

# ============================================================
# Step 0: Check prerequisites
# ============================================================
step "Step 0/7: Checking prerequisites..."

# Check Go binaries
check_binary() {
    if [ ! -f "$BIN_DIR/$1" ]; then
        error "Binary $BIN_DIR/$1 not found. Run 'make build-all' first."
        exit 1
    fi
}

check_binary "agent"
check_binary "memspace"
check_binary "agent-manager"
check_binary "agent-monitor"
check_binary "memspace-manager"
check_binary "memspace-monitor"
info "  ✅ All Go binaries found"

# Check Python
if command -v python3 &> /dev/null; then
    PYTHON_CMD="python3"
elif command -v python &> /dev/null; then
    PYTHON_CMD="python"
else
    error "Python not found. Please install Python 3.8+"
    exit 1
fi

PYTHON_VERSION=$($PYTHON_CMD --version 2>&1 | awk '{print $2}')
info "  ✅ Python found: $PYTHON_CMD ($PYTHON_VERSION)"

# Check and install Python dependencies
info "  Checking Python dependencies..."

MISSING_DEPS=()
$PYTHON_CMD -c "import flask" 2>/dev/null || MISSING_DEPS+=("flask")
$PYTHON_CMD -c "import openai" 2>/dev/null || MISSING_DEPS+=("openai")
$PYTHON_CMD -c "import numpy" 2>/dev/null || MISSING_DEPS+=("numpy")
$PYTHON_CMD -c "import httpx" 2>/dev/null || MISSING_DEPS+=("httpx")

if [ ${#MISSING_DEPS[@]} -eq 0 ]; then
    info "  ✅ All Python dependencies installed"
else
    warn "  Missing: ${MISSING_DEPS[*]}"

    if [ -f "requirements.txt" ]; then
        info "  Installing from requirements.txt..."
        $PYTHON_CMD -m pip install -r requirements.txt --quiet 2>/dev/null
    else
        info "  Installing: ${MISSING_DEPS[*]}..."
        $PYTHON_CMD -m pip install "${MISSING_DEPS[@]}" --quiet 2>/dev/null
    fi

    # Verify
    STILL_MISSING=()
    $PYTHON_CMD -c "import flask" 2>/dev/null || STILL_MISSING+=("flask")
    $PYTHON_CMD -c "import openai" 2>/dev/null || STILL_MISSING+=("openai")
    $PYTHON_CMD -c "import numpy" 2>/dev/null || STILL_MISSING+=("numpy")
    $PYTHON_CMD -c "import httpx" 2>/dev/null || STILL_MISSING+=("httpx")

    if [ ${#STILL_MISSING[@]} -ne 0 ]; then
        error "Failed to install: ${STILL_MISSING[*]}"
        error "Please run: $PYTHON_CMD -m pip install ${STILL_MISSING[*]}"
        exit 1
    fi
    info "  ✅ Python dependencies installed"
fi

# Check Python service files
CHAT_SERVER=""
EMBEDDING_SERVER=""

# Search common locations
for dir in "." "./services" "./cmd" "./pkg"; do
    [ -f "$dir/chat_server.py" ] && CHAT_SERVER="$dir/chat_server.py"
    [ -f "$dir/chat_client_server.py" ] && CHAT_SERVER="$dir/chat_client_server.py"
    [ -f "$dir/embedding_server.py" ] && EMBEDDING_SERVER="$dir/embedding_server.py"
    [ -f "$dir/embedding_server_client.py" ] && EMBEDDING_SERVER="$dir/embedding_server_client.py"
done

if [ -z "$CHAT_SERVER" ]; then
    error "chat_server.py not found. Searched: . ./services ./cmd ./pkg"
    error "Please set CHAT_SERVER variable in this script."
    exit 1
fi
if [ -z "$EMBEDDING_SERVER" ]; then
    error "embedding_server.py not found. Searched: . ./services ./cmd ./pkg"
    error "Please set EMBEDDING_SERVER variable in this script."
    exit 1
fi
info "  ✅ Chat server: $CHAT_SERVER"
info "  ✅ Embedding server: $EMBEDDING_SERVER"

# Check config files
for cfg in "agent_manager.yaml" "agent_monitor_1.yaml" "memspace_manager.yaml" "memspace_monitor_1.yaml"; do
    if [ ! -f "$CONFIG_DIR/$cfg" ]; then
        error "Config file not found: $CONFIG_DIR/$cfg"
        exit 1
    fi
done
info "  ✅ All config files found"

echo ""

# ============================================================
# Step 1: Start TinyKV cluster
# ============================================================
step "Step 1/7: Starting TinyKV cluster..."
if [ -f "./start_cluster.sh" ]; then  # 注意：是短横线 -
    # 后台执行集群脚本，并重定向输出避免干扰
    bash ./start_cluster.sh > "$LOG_DIR/tinykv-cluster.log" 2>&1 &
    CLUSTER_PID=$!
    echo $CLUSTER_PID > "$PID_DIR/tinykv-cluster.pid"

    # 等待 PD 服务就绪（最多 15 秒）
    info "  Waiting for PD server to be ready..."
    for i in {1..15}; do
        if curl -s http://127.0.0.1:2379 > /dev/null 2>&1; then
            info "  ✅ PD server is ready"
            break
        fi
        sleep 1
    done
    info "  TinyKV cluster started (PID: $CLUSTER_PID)"
else
    warn "  start_cluster.sh not found, assuming TinyKV is already running"
fi
# ============================================================
# Step 2: Start LLM Chat Server
# ============================================================
step "Step 2/7: Starting LLM Chat Server..."
nohup $PYTHON_CMD "$CHAT_SERVER" \
    > "$LOG_DIR/chat-server.log" 2>&1 &
echo $! > "$PID_DIR/chat-server.pid"
info "  Started (PID: $(cat $PID_DIR/chat-server.pid))"
sleep 3

# ============================================================
# Step 3: Start Embedding Server
# ============================================================
step "Step 3/7: Starting Embedding Server..."
nohup $PYTHON_CMD "$EMBEDDING_SERVER" \
    > "$LOG_DIR/embedding-server.log" 2>&1 &
echo $! > "$PID_DIR/embedding-server.pid"
info "  Started (PID: $(cat $PID_DIR/embedding-server.pid))"
sleep 3

# ============================================================
# Step 4: Start MemSpaceManager
# ============================================================
step "Step 4/7: Starting MemSpaceManager..."
nohup "$BIN_DIR/memspace-manager" \
    --config "$CONFIG_DIR/memspace_manager.yaml" \
    > "$LOG_DIR/memspace-manager.log" 2>&1 &
echo $! > "$PID_DIR/memspace-manager.pid"
info "  Started (PID: $(cat $PID_DIR/memspace-manager.pid))"
sleep 3

# ============================================================
# Step 5: Start MemSpaceMonitor
# ============================================================
step "Step 5/7: Starting MemSpaceMonitor..."
nohup "$BIN_DIR/memspace-monitor" \
    --config "$CONFIG_DIR/memspace_monitor_1.yaml" \
    > "$LOG_DIR/memspace-monitor.log" 2>&1 &
echo $! > "$PID_DIR/memspace-monitor.pid"
info "  Started (PID: $(cat $PID_DIR/memspace-monitor.pid))"
sleep 3

# ============================================================
# Step 6: Start AgentManager
# ============================================================
step "Step 6/7: Starting AgentManager..."
nohup "$BIN_DIR/agent-manager" \
    --config "$CONFIG_DIR/agent_manager.yaml" \
    > "$LOG_DIR/agent-manager.log" 2>&1 &
echo $! > "$PID_DIR/agent-manager.pid"
info "  Started (PID: $(cat $PID_DIR/agent-manager.pid))"
sleep 3

# ============================================================
# Step 7: Start AgentMonitor
# ============================================================
step "Step 7/7: Starting AgentMonitor..."
nohup "$BIN_DIR/agent-monitor" \
    --config "$CONFIG_DIR/agent_monitor_1.yaml" \
    > "$LOG_DIR/agent-monitor.log" 2>&1 &
echo $! > "$PID_DIR/agent-monitor.pid"
info "  Started (PID: $(cat $PID_DIR/agent-monitor.pid))"
sleep 3

# ============================================================
# Health checks
# ============================================================
echo ""
step "Health checks..."
check_service() {
    local name=$1
    local url=$2
    local max_retries=10  # 最多等 10 次
    local wait=2          # 每次等 2 秒

    for i in $(seq 1 $max_retries); do
        if curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null | grep -q "200"; then
            info "  ✅ $name"
            return 0
        fi
        sleep $wait
    done
    warn "  ⚠️  $name not responding after ${max_retries} attempts"
    warn "     Check: cat logs/${name}.log"
    warn "     PID:   cat pids/${name}.pid"
}

check_service "chat-server"      "http://localhost:20001/health"
check_service "embedding-server"  "http://localhost:20002/health"
check_service "agent-manager"    "http://localhost:7007/api/v1/agent_manager/listAgent"
check_service "memspace-manager" "http://localhost:9200/api/v1/manager/list_memspaces"

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║        🌳 NucleusMem Cluster Ready                     ║${NC}"
echo -e "${GREEN}╠══════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  LLM Chat Server:   localhost:20001                     ║${NC}"
echo -e "${GREEN}║  Embedding Server:  localhost:20002                     ║${NC}"
echo -e "${GREEN}║  AgentManager:      localhost:7007                      ║${NC}"
echo -e "${GREEN}║  AgentMonitor:      localhost:9091                      ║${NC}"
echo -e "${GREEN}║  MemSpaceManager:   localhost:9200                      ║${NC}"
echo -e "${GREEN}║  MemSpaceMonitor:   localhost:9111                      ║${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  Logs:  ./logs/     PIDs: ./pids/                      ║${NC}"
echo -e "${GREEN}║  Next:  make demo                                      ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"