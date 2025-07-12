# Goulette etcd集成部署指南

本文档描述了如何在Goulette消息队列系统中集成etcd服务，实现分布式topic副本管理和操作日志记录。

## 概述

在v0.3.0版本中，我们在每个哨兵节点内嵌入了etcd服务，用于：

1. **Topic副本管理**: 将各topic对应的副本信息保存在etcd中
2. **操作日志记录**: 记录所有涉及哈希环变更的操作，支持故障恢复
3. **Leader选举**: 自动选举etcd leader，确保数据一致性

## 架构设计

### 组件关系
```
Producer/Consumer
       ↓
   Sentinel Node (任意一个)
       ↓
   etcd Cluster
       ↓
   Leader Sentinel (处理unreachable_brokers)
```

### 数据流
1. **正常查询**: Producer/Consumer → Sentinel → etcd → 返回结果
2. **故障处理**: Producer/Consumer → Sentinel → Leader Sentinel → 更新etcd → 返回新结果

## 部署步骤

### 1. 安装etcd

#### Linux/macOS
```bash
# 下载etcd
wget https://github.com/etcd-io/etcd/releases/download/v3.5.12/etcd-v3.5.12-linux-amd64.tar.gz
tar -xzf etcd-v3.5.12-linux-amd64.tar.gz
sudo mv etcd-v3.5.12-linux-amd64/etcd /usr/local/bin/
```

#### Windows
1. 从 https://github.com/etcd-io/etcd/releases 下载Windows版本
2. 解压并将etcd.exe添加到PATH环境变量

### 2. 启动etcd集群

#### 单节点模式
```bash
# Linux/macOS
./scripts/start_etcd.sh node1 2379 2380 /tmp/etcd-data

# Windows
scripts\start_etcd.bat node1 2379 2380 C:\tmp\etcd-data
```

#### 多节点集群模式
```bash
# 节点1
./scripts/start_etcd.sh node1 2379 2380 /tmp/etcd-data-1 "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382"

# 节点2
./scripts/start_etcd.sh node2 2380 2381 /tmp/etcd-data-2 "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382"

# 节点3
./scripts/start_etcd.sh node3 2381 2382 /tmp/etcd-data-3 "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382"
```

### 3. 启动哨兵节点

```bash
# Linux/macOS
./scripts/start_sentinel.sh sentinel-1 :50052 localhost:2379

# Windows
scripts\start_sentinel.bat sentinel-1 :50052 localhost:2379
```

### 4. 验证部署

```bash
# 检查etcd状态
etcdctl endpoint status

# 检查哨兵节点日志
tail -f logs/sentinel_sentinel-1.log
```

## 配置参数

### etcd配置
- `--name`: 节点名称
- `--data-dir`: 数据存储目录
- `--listen-client-urls`: 客户端监听地址
- `--listen-peer-urls`: 对等节点监听地址
- `--initial-cluster`: 初始集群配置

### 哨兵节点配置
- `-id`: 哨兵节点ID
- `-address`: 监听地址
- `-etcd-endpoints`: etcd端点列表
- `-log-level`: 日志级别

## 工作流程

### GetBrokers流程

1. **正常查询** (unreachable_brokers为空)
   ```
   Producer/Consumer → Sentinel → etcd查询 → 返回结果
   ```

2. **故障处理** (unreachable_brokers不为空)
   ```
   Producer/Consumer → Sentinel → 检查是否为leader
   ├─ 是leader: 直接处理 → 更新etcd → 返回结果
   └─ 不是leader: 转发到leader → 处理 → 更新etcd → 返回结果
   ```

### 操作日志记录

所有涉及哈希环变更的操作都会被记录到etcd中：

- `addBrokerToHashRing`: 添加broker到哈希环
- `registerBroker`: 注册broker
- `unregisterBroker`: 下线broker
- `generateTopicReplicas`: 生成topic副本
- `handleUnreachableBrokers`: 处理无法连接的broker

### 故障恢复

哨兵节点启动时会自动从etcd恢复所有操作：

1. 读取所有操作日志
2. 按顺序重放操作
3. 重建哈希环和broker状态

## 监控和维护

### 健康检查
```bash
# 检查etcd集群状态
etcdctl endpoint health

# 检查哨兵节点状态
curl http://localhost:50052/health
```

### 数据备份
```bash
# 备份etcd数据
etcdctl snapshot save backup.db

# 恢复etcd数据
etcdctl snapshot restore backup.db
```

### 日志管理
- etcd日志: 查看etcd进程输出
- 哨兵日志: `logs/sentinel_<id>.log`
- 操作日志: 存储在etcd中，可通过API查询

## 故障排除

### 常见问题

1. **etcd连接失败**
   - 检查etcd服务是否启动
   - 验证端点地址和端口
   - 检查防火墙设置

2. **Leader选举失败**
   - 确保集群中有足够的节点
   - 检查网络连通性
   - 查看etcd日志

3. **操作恢复失败**
   - 检查etcd数据完整性
   - 验证操作日志格式
   - 查看哨兵节点日志

### 调试命令
```bash
# 查看etcd中的topic副本信息
etcdctl get /goulette/topics/test-topic

# 查看操作日志
etcdctl get /goulette/operations/ --prefix

# 查看递增ID
etcdctl get /goulette/increment/operation
```

## 性能优化

1. **etcd配置优化**
   - 调整`--quota-backend-bytes`限制存储大小
   - 配置`--auto-compaction-retention`自动清理
   - 优化`--max-request-bytes`和`--max-txn-ops`

2. **哨兵节点优化**
   - 调整健康检查间隔
   - 优化缓存策略
   - 配置合适的日志级别

## 安全考虑

1. **etcd安全**
   - 启用TLS加密
   - 配置认证和授权
   - 限制网络访问

2. **哨兵节点安全**
   - 使用gRPC TLS
   - 实现访问控制
   - 保护敏感配置

## 版本兼容性

- etcd: v3.5.x
- Go: 1.24+
- Goulette: v0.3.0+

## 升级指南

1. 备份etcd数据
2. 停止所有服务
3. 升级etcd和哨兵节点
4. 恢复数据并启动服务
5. 验证功能正常 