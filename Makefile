# Makefile

.PHONY: build-all build-agent build-memspace build-agent-manager build-agent-monitor \
        build-memspace-manager build-memspace-monitor build-demo \
        start stop restart clean help

# ============================================================
# Paths
# ============================================================
BIN_DIR := ./bin
CONFIG_DIR := ./pkg/configs/file

# ============================================================
# Build targets
# ============================================================


build-all: build-agent build-memspace build-agent-manager build-agent-monitor \
           build-memspace-manager build-memspace-monitor build-demo
	@echo "✅ All binaries built in $(BIN_DIR)/"
	@chmod +x $(BIN_DIR)/*
	@echo "🔒 Permissions set to executable."

build-agent:
	@echo "🔨 Building agent..."
	@chmod +x $(BIN_DIR)/*

	@go build -o $(BIN_DIR)/agent ./cmd/agent

build-memspace:
	@echo "🔨 Building memspace..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/memspace ./cmd/memspace

build-agent-manager:
	@echo "🔨 Building agent-manager..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/agent-manager ./cmd/agent_manager

build-agent-monitor:
	@echo "🔨 Building agent-monitor..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/agent-monitor ./cmd/agent_monitor

build-memspace-manager:
	@echo "🔨 Building memspace-manager..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/memspace-manager ./cmd/memspace_manager

build-memspace-monitor:
	@echo "🔨 Building memspace-monitor..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/memspace-monitor ./cmd/memspace_monitor

build-demo:
	@echo "🔨 Building viewspace-demo..."
	@chmod +x $(BIN_DIR)/*
	@go build -o $(BIN_DIR)/viewspace-demo ./cmd/viewspace_demo

# ============================================================
# Run targets
# ============================================================

start: build-all
	@echo "🚀 Starting NucleusMem cluster..."
	@bash start_nucleus.sh

stop:
	@echo "🛑 Stopping NucleusMem cluster..."
	@bash stop_nucleus.sh

restart: stop start

# ============================================================
# Demo
# ============================================================

demo: build-demo
	@echo "🌳 Running ViewSpaceTree demo..."
	@$(BIN_DIR)/viewspace-demo

# ============================================================
# Clean
# ============================================================

clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(BIN_DIR)/*
	@rm -rf /tmp/nucleusmem/configs/*
	@rm -rf logs/*
	@rm -rf pids/*
	@echo "✅ Clean done"

# ============================================================
# Help
# ============================================================

help:
	@echo "╔══════════════════════════════════════════════════════════╗"
	@echo "║           🌳 NucleusMem Makefile                       ║"
	@echo "╠══════════════════════════════════════════════════════════╣"
	@echo "║  make build-all    Build all binaries                   ║"
	@echo "║  make start        Build + start all services           ║"
	@echo "║  make stop         Stop all services                    ║"
	@echo "║  make restart      Stop + start                        ║"
	@echo "║  make demo         Build + run ViewSpaceTree demo       ║"
	@echo "║  make clean        Remove binaries and temp files       ║"
	@echo "║  make help         Show this help                       ║"
	@echo "╚══════════════════════════════════════════════════════════╝"