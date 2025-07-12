# Raft Leader 集成修改说明

## 概述

由于副本之间将通过raft进行同步，需要对GetBrokers服务进行重命名和更新，改为只返回topic replication的leader。

## 主要修改

### 1. Proto文件修改

**文件**: `proto/goulette.proto`

- 将 `GetBrokers` RPC 重命名为 `GetTopicLeader`
- 将 `GetBrokersRequest` 重命名为 `GetTopicLeaderRequest`
- 将 `GetBrokersResponse` 重命名为 `GetTopicLeaderResponse`
- 修改响应结构，从返回broker列表改为只返回leader

```protobuf
// 查询Topic对应的Leader Broker
rpc GetTopicLeader(GetTopicLeaderRequest) returns (GetTopicLeaderResponse);

// 获取Topic Leader请求
message GetTopicLeaderRequest {
  string topic = 1;
  uint32 replica_count = 2;
  repeated string unreachable_brokers = 3;
}

// 获取Topic Leader响应
message GetTopicLeaderResponse {
  BrokerInfo leader = 1;
  string error_message = 2;
}
```

### 2. Sentinel服务器修改

**文件**: `sentinel/server.go`

#### 2.1 ReplicationCache结构体更新
- 添加 `Leader *BrokerInfo` 字段，记录当前topic replication的leader

#### 2.2 方法重命名和更新
- `GetBrokers` → `GetTopicLeader`
- `getBrokersFromEtcd` → `getTopicLeaderFromEtcd`
- `generateAndStoreTopicReplicas` → `generateAndStoreTopicLeader`
- `handleUnreachableBrokers` → `handleUnreachableBrokersForLeader`
- `getBrokersLegacy` → `getTopicLeaderLegacy`
- `forwardToLeader` 方法参数类型更新

#### 2.3 缓存更新逻辑
- 在创建ReplicationCache时，选择第一个broker作为leader
- 所有相关方法都修改为只返回leader信息

### 3. 客户端修改

#### 3.1 Producer客户端
**文件**: `client/producer.go`

- `getBrokers` → `getTopicLeader`
- 修改返回类型从 `[]string` 改为 `string`（只返回leader地址）
- 更新 `SendMessage` 方法，只连接到leader broker

#### 3.2 Consumer客户端
**文件**: `client/consumer.go`

- `getBrokers` → `getTopicLeader`
- 修改返回类型从 `[]string` 改为 `string`（只返回leader地址）
- 更新 `StartConsuming` 和 `restartConsuming` 方法，只连接到leader broker

### 4. 测试文件更新

#### 4.1 单元测试
**文件**: `sentinel/server_test.go`

- 更新所有测试用例，使用新的 `GetTopicLeader` 方法
- 修改断言，检查leader而不是broker列表
- 验证缓存中的leader信息

#### 4.2 集成测试
**文件**: `test_etcd_integration.tomodify`

- 更新测试用例，使用新的 `GetTopicLeader` 方法
- 修改输出信息，显示leader地址

## 设计原理

### Leader选择策略
- 选择副本列表中的第一个broker作为leader
- 这个策略简单有效，确保了一致性
- 在raft实现中，可以通过选举机制动态选择leader

### 缓存一致性
- ReplicationCache现在包含leader信息
- 当副本列表更新时，leader也会相应更新
- 保持了缓存的一致性

### 向后兼容性
- 保持了原有的请求参数结构
- 客户端只需要修改方法调用，不需要修改业务逻辑
- 错误处理机制保持不变

## 影响分析

### 正面影响
1. **简化架构**: 客户端只需要连接leader，减少了连接复杂度
2. **一致性保证**: 通过raft机制确保数据一致性
3. **性能提升**: 减少了不必要的broker连接

### 需要注意的点
1. **Leader故障**: 需要实现leader故障转移机制
2. **负载均衡**: 所有请求都发送到leader，需要考虑负载均衡
3. **监控**: 需要监控leader的健康状态

## 后续工作

1. **Raft实现**: 实现完整的raft共识算法
2. **Leader选举**: 实现动态leader选举机制
3. **故障转移**: 实现leader故障时的自动转移
4. **负载均衡**: 考虑多leader或读写分离策略 