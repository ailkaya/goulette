# RemoveTopic 功能说明

## 概述

RemoveTopic 功能允许 sentinel 在处理无法连接的 broker 时，先尝试调用相关 broker 的 RemoveTopic RPC 接口。如果调用成功，目标 broker 会关闭该 topic 相关的连接并返回相应的错误，Producer/Consumer 在收到此错误后会重新向 sentinel 获取目标 broker。

## 功能特性

1. **自动 Topic 移除**: 当 broker 无法连接时，sentinel 会自动调用 RemoveTopic 接口
2. **错误处理**: Producer/Consumer 能够识别 Topic 被移除的错误并自动重试
3. **WAL 记录**: 所有 RemoveTopic 操作都会记录到 WAL 中，确保数据一致性
4. **Fragment 清理**: Topic 被移除时会自动关闭相关的 Fragment 文件

## 实现细节

### 1. Proto 定义

新增了 `RemoveTopic` RPC 接口：

```protobuf
service BrokerService {
  // 移除Topic
  rpc RemoveTopic(RemoveTopicRequest) returns (RemoveTopicResponse);
}

message RemoveTopicRequest {
  string topic = 1;
  string reason = 2;  // 移除原因
}

message RemoveTopicResponse {
  bool success = 1;
  string error_message = 2;
}
```

### 2. Broker 实现

#### RemoveTopic 方法
- 检查 Topic 是否存在
- 记录移除操作到 WAL
- 从内存中移除 Topic
- 关闭相关的 Fragment 文件

#### 消息处理优化
- `processMessage`: 检查 Topic 是否已被移除，如果已移除则返回 `RESPONSE_STATUS_RETRY`
- `PullMessage`: 检查 Topic 是否已被移除，如果已移除则返回相应错误

### 3. Sentinel 实现

#### handleUnreachableBrokers 优化
在处理无法连接的 broker 时：
1. 先尝试调用每个无法连接 broker 的 RemoveTopic 接口
2. 记录调用结果（成功或失败）
3. 继续原有的 broker 状态更新和替换逻辑

#### callRemoveTopicOnBroker 方法
- 创建到 broker 的 gRPC 连接
- 调用 RemoveTopic 接口
- 处理超时和错误情况

### 4. Client 实现

#### Producer 优化
- 检测 `RESPONSE_STATUS_RETRY` 状态
- 清除当前 broker 连接缓存
- 重新向 sentinel 获取 broker 列表
- 递归重试发送消息

#### Consumer 优化
- 检测 "Topic已被移除" 错误
- 清除当前 broker 连接缓存
- 重新向 sentinel 获取 broker 列表
- 重新开始消费流程

### 5. WAL 扩展

#### WriteRemoveTopic 方法
- 写入类型为 2 的 WAL 条目
- 将移除原因作为 payload 存储

#### Replay 优化
- 支持重放类型为 2 的 WAL 条目
- 在重放时执行 Topic 移除操作

### 6. Fragment 管理

#### CloseTopic 方法
- 关闭指定 Topic 的所有 Fragment 文件
- 从内存映射中移除 Topic
- 记录操作日志

## 使用场景

1. **Broker 故障处理**: 当某个 broker 出现故障时，sentinel 会自动移除该 broker 上的 topic
2. **负载均衡**: 在重新分配 topic 时，可以主动移除某些 broker 上的 topic
3. **维护操作**: 在系统维护时，可以手动调用 RemoveTopic 接口

## 错误处理

### Producer 错误处理
```go
if resp.Status == pb.ResponseStatus_RESPONSE_STATUS_RETRY {
    // Topic被移除，重新向sentinel获取broker
    p.window.Remove(messageID)
    utils.Infof("Topic被移除，重新向sentinel获取broker: %s", resp.ErrorMessage)
    
    // 清除当前broker连接缓存，强制重新获取
    p.mu.Lock()
    delete(p.brokerClients, brokerAddress)
    p.mu.Unlock()
    
    // 递归调用自身，重新获取broker并发送
    return p.SendMessage(topic, payload)
}
```

### Consumer 错误处理
```go
if err.Error() == fmt.Sprintf("Topic已被移除: %s", topic) {
    utils.Infof("Topic被移除，重新向sentinel获取broker: %s", err.Error())
    
    // 清除当前broker连接缓存，强制重新获取
    c.mu.Lock()
    delete(c.brokerClients, brokerAddress)
    c.mu.Unlock()
    
    // 重新获取broker并重新开始消费
    if err := c.restartConsuming(topic, consumerGroup, currentOffset); err != nil {
        utils.Errorf("重新开始消费失败: %v", err)
        time.Sleep(5 * time.Second)
    }
    return
}
```

## 测试

可以使用 `test_remove_topic.go` 文件来测试 RemoveTopic 功能：

```bash
go run test_remove_topic.go
```

测试流程：
1. 启动 sentinel 和 broker
2. 注册 broker 到 sentinel
3. 发送消息到指定 topic
4. 调用 RemoveTopic 接口移除 topic
5. 尝试再次发送消息，验证错误处理

## 注意事项

1. **并发安全**: 所有操作都使用了适当的锁机制确保并发安全
2. **超时处理**: gRPC 调用设置了合理的超时时间
3. **错误恢复**: 系统能够从各种错误状态中恢复
4. **数据一致性**: 通过 WAL 确保操作的可恢复性
5. **资源清理**: Topic 移除时会正确清理相关资源

## 未来改进

1. **批量操作**: 支持批量移除多个 Topic
2. **条件移除**: 支持基于条件的 Topic 移除
3. **监控指标**: 添加 RemoveTopic 相关的监控指标
4. **权限控制**: 添加 Topic 移除的权限控制机制 