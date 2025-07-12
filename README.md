# Goulette 分布式消息队列

Goulette 是一个基于 Go 语言开发的高性能分布式消息队列系统，采用 gRPC 进行组件间通信。

## 特性

- **高吞吐低延迟**：支持大规模消息处理
- **持久化保证**：WAL + Fragment 双重持久化
- **高可用性**：自动故障检测和恢复
- **水平扩展**：支持动态扩容
- **负载均衡**：一致性哈希智能分配
- **完善的日志系统**：支持文件输出和级别控制
- **etcd集成**：分布式topic副本管理和操作日志记录

## 系统架构

```
+------------+       +-----------------+       +----------------+
|  客户端     |<---->|  哨兵集群        |<---->|  Broker集群     |
| (生产者/消费者)|       | (服务发现/负载均衡) |       | (消息存储/处理) |
+------------+       +-----------------+       +----------------+
       ↑                     ↑                     ↑
       |                     |                     |
+------------+       +-----------------+       +----------------+
| 消息滑动窗口 |       | 一致性哈希环     |<---->| 持久化存储引擎   |
+------------+       +-----------------+       +----------------+
       ↑                     ↑                     ↑
       |                     |                     |
+------------+       +-----------------+       +----------------+
| 日志系统     |       | etcd集群        |<---->| 健康检查器     |
+------------+       +-----------------+       +----------------+
```

## 快速开始

### 1. 安装依赖

```bash
make deps
```

### 2. 构建项目

```bash
make build
```

### 3. 运行服务

#### 方式一：传统模式（无etcd）

##### 启动 Sentinel（服务发现）
```bash
make run-sentinel
```

##### 启动 Broker（消息存储）
```bash
make run-broker
```

##### 发送消息
```bash
make run-producer
```

##### 消费消息
```bash
make run-consumer
```

#### 方式二：etcd集成模式（推荐）

##### 启动 etcd 服务
```bash
make run-etcd
```

##### 启动 Sentinel（带etcd）
```bash
make run-sentinel-with-etcd
```

##### 启动 Broker（消息存储）
```bash
make run-broker
```

##### 发送消息
```bash
make run-producer
```

##### 消费消息
```bash
make run-consumer
```

#### 方式三：内嵌etcd模式（推荐，无需单独安装etcd）

##### 启动 Sentinel（带内嵌etcd）
```bash
make run-sentinel-embedded
```

##### 启动 Broker（消息存储）
```bash
make run-broker
```

##### 发送消息
```bash
make run-producer
```

##### 消费消息
```bash
make run-consumer
```

### 4. 一键运行所有服务

#### 传统模式
```bash
make run-all
```

#### 外部etcd集成模式
```bash
make run-all-with-etcd
```

#### 内嵌etcd模式（推荐）
```bash
make run-all-embedded
```

### 5. 日志系统演示

```bash
go run examples/logger_example.go
```

### 6. etcd集成测试

```bash
# 方式一：使用外部etcd
make run-etcd &
make run-sentinel-with-etcd &
go run test_etcd_integration.go

# 方式二：使用内嵌etcd（推荐）
make test-embedded-etcd
```

## etcd集成特性

### 核心功能
- **Topic副本管理**：将各topic对应的副本信息保存在etcd中
- **操作日志记录**：记录所有涉及哈希环变更的操作，支持故障恢复
- **Leader选举**：自动选举etcd leader，确保数据一致性
- **故障恢复**：哨兵节点启动时自动从etcd恢复所有操作
- **内嵌etcd支持**：无需单独安装etcd，sentinel内置etcd服务

### 部署模式

#### 1. 内嵌etcd模式（推荐）
- **优势**：
  - 无需单独安装etcd
  - 部署简单，一键启动
  - 自动管理etcd生命周期
  - 适合单节点或小规模集群
- **适用场景**：开发测试、单节点部署、小规模生产环境

#### 2. 外部etcd模式
- **优势**：
  - 独立的etcd集群，更稳定
  - 支持大规模集群
  - 可以复用现有的etcd基础设施
- **适用场景**：大规模生产环境、已有etcd基础设施

### 工作流程

#### GetBrokers流程
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

### 集群部署

#### 启动内嵌etcd哨兵集群（3节点）
```bash
make run-sentinel-cluster-embedded
```

#### 启动外部etcd哨兵集群（3节点）
```bash
make run-etcd-cluster
make run-sentinel-cluster
```

### etcd管理命令
```bash
# 检查etcd状态
make etcd-status

# 查看topic副本信息
make etcd-topics

# 查看操作日志
make etcd-operations

# 清理etcd数据
make etcd-clean

# 备份etcd数据
make etcd-backup

# 恢复etcd数据
make etcd-restore
```

详细部署说明请参考：[etcd集成部署指南](docs/etcd_integration.md)

## 项目结构

```
goulette/
├── cmd/                    # 命令行工具
│   ├── broker/            # Broker 服务
│   ├── sentinel/          # Sentinel 服务
│   └── client/            # 客户端工具
├── internal/              # 内部包
│   ├── etcd/              # etcd管理器
│   ├── storage/           # 存储引擎
│   └── utils/             # 工具函数
├── broker/                # Broker 模块
├── sentinel/              # Sentinel 模块
├── client/                # 客户端模块
├── proto/                 # Protocol Buffers 定义
├── scripts/               # 启动脚本
├── docs/                  # 文档
├── examples/              # 示例程序
├── logs/                  # 日志文件目录
└── Makefile              # 构建脚本
```

## 核心模块

### 1. Broker 模块
- **功能**：消息存储和处理
- **特性**：
  - WAL（预写日志）保证数据安全
  - Fragment 分片存储提高性能
  - 支持消息持久化和恢复
  - 自动日志记录到 `logs/broker_<id>.log`

### 2. Sentinel 模块
- **功能**：服务发现和负载均衡
- **特性**：
  - 一致性哈希环分配
  - 健康检查和故障检测
  - 自动故障转移
  - Broker缓存机制
  - etcd集成支持
  - 自动日志记录到 `logs/sentinel_<id>.log`

### 3. etcd管理器
- **功能**：分布式数据管理
- **特性**：
  - Topic副本信息存储
  - 操作日志记录
  - Leader选举
  - 故障恢复支持
  - 原子递增ID生成

### 4. 客户端模块
- **功能**：生产者和消费者
- **特性**：
  - 滑动窗口控制并发
  - 异步消息发送
  - 自动重连和错误处理
  - 自动日志记录到 `logs/producer.log` 和 `logs/consumer.log`

### 5. 日志系统
- **功能**：统一的日志管理
- **特性**：
  - 支持文件和控制台输出
  - 5个日志级别：DEBUG、INFO、WARN、ERROR、FATAL
  - 自动创建日志目录
  - 线程安全的并发写入
  - 格式化日志消息
  - 错误详情记录

## 日志系统使用

### 初始化日志
```go
// 文件日志
err := utils.InitLogger("info", "logs/app.log")

// 控制台日志
err := utils.InitLogger("debug", "")
```

### 日志级别
```go
utils.Debug("调试信息")
utils.Info("一般信息")
utils.Warn("警告信息")
utils.Error("错误信息")
utils.Fatal("致命错误") // 会退出程序
```

### 格式化日志
```go
utils.Infof("用户 %s 登录成功", "张三")
utils.Errorf("处理失败: %v", err)
```

### 错误日志
```go
utils.ErrorWithErr(err, "操作失败")
```

### 日志控制
```go
utils.SetLogLevel("warn") // 只显示WARN及以上级别
utils.Close() // 关闭日志系统
```

## 配置参数

### Broker 参数
- `-id`: Broker ID
- `-address`: 监听地址
- `-data`: 数据目录
- `-log-level`: 日志级别

### Sentinel 参数
- `-id`: Sentinel ID
- `-address`: 监听地址
- `-etcd-endpoints`: 外部etcd端点列表（当enable-embedded-etcd=false时使用）
- `-enable-embedded-etcd`: 是否启用内嵌etcd服务（默认true）
- `-etcd-port`: 内嵌etcd客户端端口（默认2379）
- `-etcd-peer-port`: 内嵌etcd对等端口（默认2380）
- `-etcd-data-dir`: 内嵌etcd数据目录（默认default.etcd）
- `-log-level`: 日志级别

### 客户端参数
- `-mode`: 运行模式（producer/consumer）
- `-sentinel`: Sentinel 地址
- `-topic`: Topic 名称
- `-message`: 要发送的消息
- `-group`: 消费者组
- `-window`: 滑动窗口大小

## 开发

### 运行测试
```bash
make test
```

### 代码格式化
```bash
make fmt
```

### 代码检查
```bash
make vet
```

### 清理构建文件
```bash
make clean
```

### 停止所有服务
```bash
make stop-all
```

## 版本历史

### v0.3.0
- ✅ 集成etcd服务
- ✅ 实现分布式topic副本管理
- ✅ 添加操作日志记录功能
- ✅ 支持故障恢复
- ✅ 实现Leader选举机制
- ✅ 提供完整的部署脚本和文档

### v0.2.0
- ✅ 实现Broker模块
- ✅ 实现Sentinel模块
- ✅ 实现客户端模块
- ✅ 添加日志系统
- ✅ 实现健康检查

### v0.1.0
- ✅ 基础架构设计
- ✅ gRPC通信框架
- ✅ 一致性哈希环

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

