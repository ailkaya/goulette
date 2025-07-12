# Etcd Lease 心跳机制

## 概述

本项目已更新 `KeepAlive` 函数，现在使用 etcd 的 lease 机制来实现 broker 心跳服务。这种实现方式具有以下优势：

1. **自动过期清理**：当 broker 停止发送心跳时，etcd 会自动清理相关数据
2. **分布式一致性**：多个 sentinel 节点可以共享相同的 broker 状态信息
3. **实时通知**：通过 etcd 的 watch 机制，可以实时感知 broker 的上下线
4. **高可用性**：即使某个 sentinel 节点宕机，其他节点仍能正常工作

## 实现细节

### 1. EtcdManager 新增功能

在 `internal/etcd/etcd.go` 中新增了以下方法：

- `RegisterBrokerHeartbeat(brokerID, address string, ttl int64) error`：注册 broker 心跳
- `RenewBrokerHeartbeat(brokerID, address string, ttl int64) error`：续约 broker 心跳
- `UnregisterBrokerHeartbeat(brokerID string) error`：注销 broker 心跳
- `GetBrokerHeartbeats() ([]BrokerInfo, error)`：获取所有 broker 心跳信息
- `WatchBrokerHeartbeats(handler func(brokerID string, eventType string, broker *BrokerInfo))`：监听 broker 心跳变化

### 2. KeepAlive 函数更新

`KeepAlive` 函数现在的工作流程：

1. 如果 etcd 管理器可用，使用 etcd lease 机制
2. 设置 TTL 为 30 秒
3. 尝试续约现有心跳，如果不存在则注册新的
4. 同时更新本地 broker 信息（用于兼容性）
5. 如果 etcd 不可用，回退到原来的实现

### 3. 自动监听机制

Sentinel 服务器启动时会：

1. 启动 etcd 心跳监听
2. 从 etcd 恢复现有的 broker 信息
3. 实时处理 broker 的上下线事件

## 使用方法

### 1. 启动 etcd

确保 etcd 服务正在运行：

```bash
# 使用提供的脚本启动 etcd
./scripts/start_etcd.sh
# 或者在 Windows 上
./scripts/start_etcd.bat
```

### 2. 配置 Sentinel

在启动 sentinel 时指定 etcd 端点：

```go
sentinel := sentinel.NewSentinelServer("sentinel-001", ":8080", []string{"localhost:2379"})
```

### 3. 测试心跳机制

运行测试程序验证功能：

```bash
go run test_etcd_heartbeat.go
```

## 配置参数

### TTL 设置

当前 TTL 设置为 30 秒，可以根据需要调整：

```go
// 在 KeepAlive 函数中
ttl := int64(30) // 可以根据需要调整
```

### 心跳前缀

Broker 心跳信息存储在 etcd 中的前缀：

```go
const BrokerHeartbeatPrefix = "/goulette/brokers/"
```

## 故障处理

### 1. Etcd 不可用

如果 etcd 不可用，系统会自动回退到原来的实现方式，确保服务的连续性。

### 2. 网络分区

在网络分区情况下，etcd 的 lease 机制会自动处理：
- 如果 broker 无法连接到 etcd，其心跳会过期
- 其他 sentinel 节点会检测到 broker 下线
- 当网络恢复时，broker 可以重新注册

### 3. 数据一致性

通过 etcd 的事务机制确保数据一致性：
- 使用事务确保 lease 创建和数据写入的原子性
- 使用 watch 机制确保状态变化的实时性

## 监控和调试

### 1. 日志输出

系统会输出详细的日志信息：

```
INFO: 启动etcd心跳监听...
INFO: etcd心跳监听启动成功
INFO: 从etcd恢复broker信息成功
DEBUG: Broker心跳续约成功: broker-001 (localhost:9090)
INFO: 从etcd检测到新broker上线: broker-001 (localhost:9090)
WARN: 从etcd检测到broker下线: broker-001
```

### 2. 状态查询

可以通过 etcd 客户端查询当前状态：

```bash
# 查看所有 broker 心跳
etcdctl get /goulette/brokers/ --prefix

# 查看特定 broker
etcdctl get /goulette/brokers/broker-001
```

## 性能考虑

### 1. Lease 管理

- 每个 broker 对应一个 lease
- lease 的 TTL 设置为 30 秒，平衡了实时性和性能
- 使用 `KeepAliveOnce` 进行续约，减少网络开销

### 2. Watch 机制

- 使用 etcd 的 watch 机制监听变化
- 避免轮询，提高效率
- 支持断线重连

### 3. 内存使用

- 本地维护 broker 信息用于快速访问
- etcd 中存储完整的状态信息
- 定期清理过期的本地数据

## 未来改进

1. **批量操作**：支持批量注册/注销 broker
2. **自定义 TTL**：允许不同 broker 使用不同的 TTL
3. **健康检查**：结合 etcd 的 lease 机制和主动健康检查
4. **指标收集**：添加心跳延迟、成功率等指标 