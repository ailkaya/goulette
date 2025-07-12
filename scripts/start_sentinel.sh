#!/bin/bash

# 哨兵节点启动脚本
# 用法: ./start_sentinel.sh <sentinel_id> <sentinel_port> <etcd_endpoints>

set -e

if [ $# -lt 3 ]; then
    echo "用法: $0 <sentinel_id> <sentinel_port> <etcd_endpoints>"
    echo "示例: $0 sentinel-1 :50052 localhost:2379"
    echo "示例: $0 sentinel-1 :50052 localhost:2379,localhost:2380"
    exit 1
fi

SENTINEL_ID=$1
SENTINEL_PORT=$2
ETCD_ENDPOINTS=$3

# 检查Go是否已安装
if ! command -v go &> /dev/null; then
    echo "错误: Go未安装，请先安装Go"
    echo "安装方法: https://golang.org/dl/"
    exit 1
fi

# 切换到项目根目录
cd "$(dirname "$0")/.."

# 构建项目
echo "构建项目..."
go build -o bin/sentinel cmd/sentinel/main.go

# 启动哨兵节点
echo "启动哨兵节点: $SENTINEL_ID"
echo "监听端口: $SENTINEL_PORT"
echo "etcd端点: $ETCD_ENDPOINTS"

./bin/sentinel \
    -id="$SENTINEL_ID" \
    -address="$SENTINEL_PORT" \
    -etcd-endpoints="$ETCD_ENDPOINTS" \
    -log-level="info" 