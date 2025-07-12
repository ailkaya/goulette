@echo off
REM etcd启动脚本 (Windows版本)
REM 用法: start_etcd.bat <node_id> <client_port> <peer_port> <data_dir> [initial_cluster]

setlocal enabledelayedexpansion

if %4=="" (
    echo 用法: %0 ^<node_id^> ^<client_port^> ^<peer_port^> ^<data_dir^> [initial_cluster]
    echo 示例: %0 node1 2379 2380 C:\tmp\etcd-data
    echo 示例: %0 node1 2379 2380 C:\tmp\etcd-data "node1=http://localhost:2380,node2=http://localhost:2381"
    exit /b 1
)

set NODE_ID=%1
set CLIENT_PORT=%2
set PEER_PORT=%3
set DATA_DIR=%4
set INITIAL_CLUSTER=%5

if "%INITIAL_CLUSTER%"=="" (
    set INITIAL_CLUSTER=%NODE_ID%=http://localhost:%PEER_PORT%
)

REM 创建数据目录
if not exist "%DATA_DIR%" mkdir "%DATA_DIR%"

REM 检查etcd是否已安装
where etcd >nul 2>&1
if %errorlevel% neq 0 (
    echo 错误: etcd未安装，请先安装etcd
    echo 安装方法: https://github.com/etcd-io/etcd/releases
    exit /b 1
)

echo 启动etcd节点: %NODE_ID%
echo 客户端端口: %CLIENT_PORT%
echo 对等端口: %PEER_PORT%
echo 数据目录: %DATA_DIR%
echo 初始集群: %INITIAL_CLUSTER%

REM 启动etcd
etcd ^
    --name="%NODE_ID%" ^
    --data-dir="%DATA_DIR%" ^
    --listen-client-urls="http://localhost:%CLIENT_PORT%" ^
    --advertise-client-urls="http://localhost:%CLIENT_PORT%" ^
    --listen-peer-urls="http://localhost:%PEER_PORT%" ^
    --initial-advertise-peer-urls="http://localhost:%PEER_PORT%" ^
    --initial-cluster="%INITIAL_CLUSTER%" ^
    --initial-cluster-state="new" ^
    --initial-cluster-token="goulette-etcd-cluster" ^
    --auto-compaction-mode="revision" ^
    --auto-compaction-retention="1000" ^
    --quota-backend-bytes="8589934592" ^
    --max-request-bytes="1048576" ^
    --max-txn-ops="128" ^
    --log-level="info" 