# Goulette 快速开始指南

本指南将帮助您快速启动和测试Goulette消息队列系统。

## 前置要求

- Go 1.24+
- Git

## 1. 克隆项目

```bash
git clone <repository-url>
cd goulette
```

## 2. 安装依赖

```bash
make deps
```

## 3. 构建项目

```bash
make build
```

## 4. 快速启动（推荐方式）

### 方式一：内嵌etcd模式（最简单）

```bash
# 一键启动所有服务（包括内嵌etcd）
make run-all-embedded
```

这个命令会：
1. 启动内嵌etcd服务
2. 启动sentinel服务
3. 启动broker服务
4. 发送测试消息
5. 消费测试消息

### 方式二：分步启动

```bash
# 1. 启动sentinel（带内嵌etcd）
make run-sentinel-embedded

# 2. 在另一个终端启动broker
make run-broker

# 3. 在另一个终端发送消息
make run-producer

# 4. 在另一个终端消费消息
make run-consumer
```

## 5. 验证功能

### 检查服务状态

```bash
# 检查etcd状态
make etcd-status

# 查看topic副本信息
make etcd-topics

# 查看操作日志
make etcd-operations
```

### 运行集成测试

```bash
# 测试内嵌etcd集成
make test-embedded-etcd
```

## 6. 停止服务

```bash
make stop-all
```

## 7. 清理数据

```bash
make clean
```

## 常见问题

### Q: 端口被占用怎么办？
A: 修改启动参数中的端口号：
```bash
./bin/sentinel -id sentinel-1 -address :50053 -etcd-port 2381 -etcd-peer-port 2382
```

### Q: 如何查看日志？
A: 日志文件位于 `logs/` 目录：
```bash
tail -f logs/sentinel_sentinel-1.log
tail -f logs/broker_broker-1.log
```

### Q: 如何配置集群模式？
A: 参考 [etcd集成部署指南](etcd_integration.md) 中的集群部署部分。

### Q: 如何自定义配置？
A: 参考 `configs/sentinel.yaml` 配置文件示例。

## 下一步

- 阅读 [etcd集成部署指南](etcd_integration.md) 了解详细配置
- 查看 [API文档](api.md) 了解编程接口
- 参与 [贡献指南](contributing.md) 帮助改进项目 