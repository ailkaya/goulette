# Goulette
一个基于内存的高性能分布式消息队列

## Features
- 基于内存传输
- 使用gRPC进行组件间通信
- 借助etcd构建哨兵集群，提供服务发现与配置共享功能
- 基于一致性哈希进行负载均衡
- 提供多partition分片传输功能
- 消息持久化
- 暂无副本机制

### 部分设计细节
- [概览](docs/具体设计.md)
- [一致性哈希](docs/一致性哈希.png)
- [基本架构](docs/基本架构.png)
- [持久化](docs/持久化.png)
- [部分异常情况处理](docs/部分异常情况处理.png)

### TODO
- 持久化
- 部分const量迁移至配置文件
- 哨兵leader, follower职责分离
- 结构优化
- 解决producer/consumer中新旧连接交替所带来的问题
