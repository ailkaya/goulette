@echo off
REM 哨兵节点启动脚本 (Windows版本)
REM 用法: start_sentinel.bat <sentinel_id> <sentinel_port> <etcd_endpoints>

setlocal enabledelayedexpansion

if %3=="" (
    echo 用法: %0 ^<sentinel_id^> ^<sentinel_port^> ^<etcd_endpoints^>
    echo 示例: %0 sentinel-1 :50052 localhost:2379
    echo 示例: %0 sentinel-1 :50052 localhost:2379,localhost:2380
    exit /b 1
)

set SENTINEL_ID=%1
set SENTINEL_PORT=%2
set ETCD_ENDPOINTS=%3

REM 检查Go是否已安装
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo 错误: Go未安装，请先安装Go
    echo 安装方法: https://golang.org/dl/
    exit /b 1
)

REM 切换到项目根目录
cd /d "%~dp0\.."

REM 构建项目
echo 构建项目...
go build -o bin\sentinel.exe cmd\sentinel\main.go

REM 启动哨兵节点
echo 启动哨兵节点: %SENTINEL_ID%
echo 监听端口: %SENTINEL_PORT%
echo etcd端点: %ETCD_ENDPOINTS%

bin\sentinel.exe ^
    -id="%SENTINEL_ID%" ^
    -address="%SENTINEL_PORT%" ^
    -etcd-endpoints="%ETCD_ENDPOINTS%" ^
    -log-level="info" 