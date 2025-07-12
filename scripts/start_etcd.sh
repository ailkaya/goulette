#!/bin/bash

# etcd启动脚本
# 用法: ./start_etcd.sh <node_id> <client_port> <peer_port> <data_dir> [initial_cluster]

set -e

if [ $# -lt 4 ]; then
    echo "用法: $0 <node_id> <client_port> <peer_port> <data_dir> [initial_cluster]"
    echo "示例: $0 node1 2379 2380 /tmp/etcd-data"
    echo "示例: $0 node1 2379 2380 /tmp/etcd-data 'node1=http://localhost:2380,node2=http://localhost:2381'"
    exit 1
fi

NODE_ID=$1
CLIENT_PORT=$2
PEER_PORT=$3
DATA_DIR=$4
INITIAL_CLUSTER=${5:-"$NODE_ID=http://localhost:$PEER_PORT"}

# 创建数据目录
mkdir -p "$DATA_DIR"

# 检查etcd是否已安装
if ! command -v etcd &> /dev/null; then
    echo "错误: etcd未安装，请先安装etcd"
    echo "安装方法: https://github.com/etcd-io/etcd/releases"
    exit 1
fi

echo "启动etcd节点: $NODE_ID"
echo "客户端端口: $CLIENT_PORT"
echo "对等端口: $PEER_PORT"
echo "数据目录: $DATA_DIR"
echo "初始集群: $INITIAL_CLUSTER"

# 启动etcd
etcd \
    --name="$NODE_ID" \
    --data-dir="$DATA_DIR" \
    --listen-client-urls="http://localhost:$CLIENT_PORT" \
    --advertise-client-urls="http://localhost:$CLIENT_PORT" \
    --listen-peer-urls="http://localhost:$PEER_PORT" \
    --initial-advertise-peer-urls="http://localhost:$PEER_PORT" \
    --initial-cluster="$INITIAL_CLUSTER" \
    --initial-cluster-state="new" \
    --initial-cluster-token="goulette-etcd-cluster" \
    --auto-compaction-mode="revision" \
    --auto-compaction-retention="1000" \
    --quota-backend-bytes="8589934592" \
    --max-request-bytes="1048576" \
    --max-txn-ops="128" \
    --log-level="info" 