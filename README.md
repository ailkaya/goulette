### 该项目名称变更为 goulette , 且版本号重置为v0.1 ###

### v1.0.0更新计划 ###
- 使用一致性哈希作为负载均衡算法
- 新增哨兵集群维护哈希环信息
- 基于哨兵集群实现注册中心，借助etcd组件进行服务注册与发现以及哈希环操作同步
- [设计文档](/docs/)

### 具体实现方案 ###
#### 一致性哈希算法 ####
- 使用跳表作为底层数据结构，实现哈希环
- broker注册时: 根据预先设置好的分片数，将broker节点均匀地分布到哈希环上
- topic注册时: 同样根据预先设置好的分片数，将topic均匀地分布到哈希环上，设置为另外一个节点类型(这里同样将topic也插入到环上的原因是以后可能需要对整个哈希环进行优化，优化时需要知道哈希环上topic的分布情况)

#### 各对象初始化时 ####
- producer
  1. 调用哨兵集群leader的rpc接口(携带topic信息)
  2. 若topic未注册，则进行注册
  3. 哨兵leader根据topic信息，生成多个topic分片(类似topic-1, topic-2...)，然后分别哈希得到一个uint64类型的值pos，根据pos找到对应的broker地址，最后聚合结果返回
  4. producer根据返回的broker地址，分别建立连接并开始传输
  5. producer与broker之间的连接由producer主动建立，但之后的数据传输由broker主动发起
- consumer(同producer)
- broker
  1. 调用哨兵集群leader的rpc接口(携带broker地址信息)
  2. 若broker未注册，则进行注册; 若已注册，则将状态置为1后返回
  3. 哨兵leader根据broker地址信息，生成多个broker分片(类似broker-1, broker-2...)，然后分别哈希得到一个uint64类型的值pos，根据pos插入broker节点到哈希环上
  4. consumer根据返回的broker地址，分别建立连接并开始传输
  5. broker与consumer之间的连接由consumer主动建立，并且之后的数据传输由consumer主动发起
 
#### 哨兵集群 ####
- 哨兵节点本身是一个grpc服务端，通过嵌入etcd实现leader选举和集群信息同步
- 每次broker/topic注册时，leader会向etcd中写入一条命令，集群中follower监听相关key，若有新增命令则执行该命令

#### 持久化 ####
- broker
  1. 支持同步写盘和异步写盘
  2. 每次接收到消息后，先将消息同步/异步写入磁盘，然后将消息放入内存中的对应channel中
  3. 采用顺序写的方式，所有topic都写入同一个文件(.msg)，每个文件最大1G，写满后新建一个文件
  4. 对于每个消息记录文件，写满后，新建一个对应的索引文件(.idx)，记录此.msg文件中各topic的起始序列号，方便后续系统恢复
  5. .lte 文件记录各topic的消费进度
  6. 系统宕机/服务下线，之后应用恢复时，先读.lte文件，获取各topic的消费进度，然后读.idx文件，找到每个topic各自对应的.msg文件，最后从各.msg文件中读取消息到内存，恢复到对应位置
- 哨兵节点
  1. 直接将每个操作命令持久化到etcd中
  2. 恢复时，直接从etcd中读取命令，然后执行
  3. 操作日志需要定期压缩，防止恢复时间过长

### 错误情况模拟 ###
- 某个broker中的某个topic仍有消息未消费但无接收者
  1. producerA.topic1 -> brokerA.topic1 -> consumerA.topic1
  2. brokerA crash
  3. brokerB插入topic1至brokerA之间的位置导致producerA.topic1 -> brokerB.topic1 -> consumerA.topic1
  4. producer从哨兵处拉取信息更新目的broker，导致producerA.topic1 -> brokerB.topic1 -> consumerA.topic1
  5. brokerA重启恢复，但此时brokerA不可能会被producerA或consumerA获取到，最终导致这部分消息无法被消费
  6. 解决方案：哨兵节点额外维护一个DAG和一个全局消费队列，记录producer,broker,consumer之间topic的依赖关系，若某个broker.topic的最后一条入度为非正常断开，则将该broker.topic放入全局消费队列中的对应位置中，当有producer/consumer根据对应topic查询broker时，就把该broker.topic从全局消费队列中取出，并更新DAG，从全局消费队列中移除，最后将其添加到返回给producer/consumer的结果中。通过该方法，可以避免某个broker.topic无consumer消费的情况