@echo off
setlocal

echo [INFO] Starting TinyKV Cluster...

set PD_SRC=.\tinykv\scheduler
set KV_SRC=.\tinykv\kv\main.go
set BIN_DIR=bin

if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"

echo [INFO] Compiling PD Server...
go build -o %BIN_DIR%\pd-server.exe %PD_SRC%
if %errorlevel% neq 0 (
    echo [ERROR] Failed to compile PD Server. Check path: %PD_SRC%
    exit /b %errorlevel%
)

echo [INFO] Compiling TinyKV Server...
go build -o %BIN_DIR%\tinykv-server.exe %KV_SRC%
if %errorlevel% neq 0 (
    echo [ERROR] Failed to compile TinyKV Server. Check path: %KV_SRC%
    exit /b %errorlevel%
)

echo [INFO] Cleaning up data...
:: 新增：清理 ./data 目录
if exist "data" (
    echo [INFO] Removing existing data directory...
    rmdir /s /q data 2>nul
)
if exist "default.pd-LAPTOP-HAF62V95" (
    echo [INFO] Removing existing config directory...
    rmdir /s /q default.pd-LAPTOP-HAF62V95 2>nul
)
echo [INFO] Starting PD...
start "PD-Server" %BIN_DIR%\pd-server.exe
timeout /t 5 >nul

echo [INFO] Starting Store 1...
start "Store-1" %BIN_DIR%\tinykv-server.exe -addr="127.0.0.1:30160" -path="./data/kv1" -scheduler="127.0.0.1:2379" -loglevel="debug"
timeout /t 5 >nul

echo [INFO] Starting Store 2...
start "Store-2" %BIN_DIR%\tinykv-server.exe -addr="127.0.0.1:30161" -path="./data/kv2" -scheduler="127.0.0.1:2379" -loglevel="debug"
timeout /t 2 >nul

echo [INFO] Starting Store 3...
start "Store-3" %BIN_DIR%\tinykv-server.exe -addr="127.0.0.1:30162" -path="./data/kv3" -scheduler="127.0.0.1:2379" -loglevel="debug"

echo [SUCCESS] Cluster started.
pause