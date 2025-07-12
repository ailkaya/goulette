.PHONY: build clean run-broker run-sentinel run-producer run-consumer test run-logger-example test-logger run-etcd run-sentinel-with-etcd run-all-with-etcd run-sentinel-embedded run-all-embedded

# 构建所有可执行文件
build:
	go build -o bin/broker cmd/broker/main.go
	go build -o bin/sentinel cmd/sentinel/main.go
	go build -o bin/client cmd/client/main.go

# 清理构建文件
clean:
	rm -rf bin/
	rm -rf data/
	rm -rf logs/
	rm -rf test_logs/
	rm -rf etcd-data/
	rm -rf default.etcd/

# 运行Broker
run-broker:
	./bin/broker -id broker-1 -address :50051 -data ./data/broker -log-level info

# 运行Sentinel (无etcd)
run-sentinel:
	./bin/sentinel -id sentinel-1 -address :50052 -log-level info

# 运行Sentinel (带外部etcd)
run-sentinel-with-etcd:
	./bin/sentinel -id sentinel-1 -address :50052 -etcd-endpoints localhost:2379 -log-level info

# 运行Sentinel (带内嵌etcd)
run-sentinel-embedded:
	./bin/sentinel -id sentinel-1 -address :50052 -enable-embedded-etcd=true -etcd-port 2379 -etcd-peer-port 2380 -etcd-data-dir ./default.etcd -log-level info

# 运行etcd (单节点)
run-etcd:
	mkdir -p etcd-data
	etcd --name node1 --data-dir etcd-data --listen-client-urls http://localhost:2379 --advertise-client-urls http://localhost:2379 --listen-peer-urls http://localhost:2380 --initial-advertise-peer-urls http://localhost:2380 --initial-cluster node1=http://localhost:2380 --initial-cluster-state new --initial-cluster-token goulette-etcd-cluster

# 运行生产者
run-producer:
	./bin/client -mode producer -sentinel localhost:50052 -topic test-topic -message "Hello Goulette!"

# 运行消费者
run-consumer:
	./bin/client -mode consumer -sentinel localhost:50052 -topic test-topic -group test-group

# 运行日志系统演示
run-logger-example:
	go run examples/logger_example.go

# 运行测试
test:
	go test ./...

# 运行日志系统测试
test-logger:
	go test ./internal/utils -v

# 安装依赖
deps:
	go mod tidy
	go mod download

# 格式化代码
fmt:
	go fmt ./...

# 检查代码
vet:
	go vet ./...

# 运行所有服务（后台，无etcd）
run-all: build
	./bin/sentinel -id sentinel-1 -address :50052 -log-level info &
	./bin/broker -id broker-1 -address :50051 -data ./data/broker -log-level info &
	sleep 2
	./bin/client -mode producer -sentinel localhost:50052 -topic test-topic -message "Hello Goulette!"
	./bin/client -mode consumer -sentinel localhost:50052 -topic test-topic -group test-group

# 运行所有服务（后台，带外部etcd）
run-all-with-etcd: build
	./bin/sentinel -id sentinel-1 -address :50052 -etcd-endpoints localhost:2379 -log-level info &
	./bin/broker -id broker-1 -address :50051 -data ./data/broker -log-level info &
	sleep 2
	./bin/client -mode producer -sentinel localhost:50052 -topic test-topic -message "Hello Goulette!"
	./bin/client -mode consumer -sentinel localhost:50052 -topic test-topic -group test-group

# 运行所有服务（后台，带内嵌etcd）
run-all-embedded: build
	./bin/sentinel -id sentinel-1 -address :50052 -enable-embedded-etcd=true -etcd-port 2379 -etcd-peer-port 2380 -etcd-data-dir ./default.etcd -log-level info &
	./bin/broker -id broker-1 -address :50051 -data ./data/broker -log-level info &
	sleep 5
	./bin/client -mode producer -sentinel localhost:50052 -topic test-topic -message "Hello Goulette!"
	./bin/client -mode consumer -sentinel localhost:50052 -topic test-topic -group test-group

# 停止所有服务
stop-all:
	pkill -f "bin/sentinel" || true
	pkill -f "bin/broker" || true
	pkill -f "bin/client" || true
	pkill -f "etcd" || true

# 检查etcd状态
etcd-status:
	etcdctl endpoint status

# 查看etcd中的topic副本信息
etcd-topics:
	etcdctl get /goulette/topics/ --prefix

# 查看etcd中的操作日志
etcd-operations:
	etcdctl get /goulette/operations/ --prefix

# 清理etcd数据
etcd-clean:
	etcdctl del /goulette/topics/ --prefix
	etcdctl del /goulette/operations/ --prefix
	etcdctl del /goulette/increment/ --prefix

# 备份etcd数据
etcd-backup:
	etcdctl snapshot save etcd-backup.db

# 恢复etcd数据
etcd-restore:
	etcdctl snapshot restore etcd-backup.db

# 启动etcd集群（3节点）
run-etcd-cluster:
	mkdir -p etcd-data-1 etcd-data-2 etcd-data-3
	etcd --name node1 --data-dir etcd-data-1 --listen-client-urls http://localhost:2379 --advertise-client-urls http://localhost:2379 --listen-peer-urls http://localhost:2380 --initial-advertise-peer-urls http://localhost:2380 --initial-cluster "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382" --initial-cluster-state new --initial-cluster-token goulette-etcd-cluster &
	etcd --name node2 --data-dir etcd-data-2 --listen-client-urls http://localhost:2380 --advertise-client-urls http://localhost:2380 --listen-peer-urls http://localhost:2381 --initial-advertise-peer-urls http://localhost:2381 --initial-cluster "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382" --initial-cluster-state new --initial-cluster-token goulette-etcd-cluster &
	etcd --name node3 --data-dir etcd-data-3 --listen-client-urls http://localhost:2381 --advertise-client-urls http://localhost:2381 --listen-peer-urls http://localhost:2382 --initial-advertise-peer-urls http://localhost:2382 --initial-cluster "node1=http://localhost:2380,node2=http://localhost:2381,node3=http://localhost:2382" --initial-cluster-state new --initial-cluster-token goulette-etcd-cluster &

# 启动哨兵集群（3节点，带etcd）
run-sentinel-cluster: build
	./bin/sentinel -id sentinel-1 -address :50052 -etcd-endpoints localhost:2379,localhost:2380,localhost:2381 -log-level info &
	./bin/sentinel -id sentinel-2 -address :50053 -etcd-endpoints localhost:2379,localhost:2380,localhost:2381 -log-level info &
	./bin/sentinel -id sentinel-3 -address :50054 -etcd-endpoints localhost:2379,localhost:2380,localhost:2381 -log-level info &

# 启动哨兵集群（3节点，带内嵌etcd）
run-sentinel-cluster-embedded: build
	./bin/sentinel -id sentinel-1 -address :50052 -enable-embedded-etcd=true -etcd-port 2379 -etcd-peer-port 2380 -etcd-data-dir ./default.etcd-1 -log-level info &
	./bin/sentinel -id sentinel-2 -address :50053 -enable-embedded-etcd=true -etcd-port 2381 -etcd-peer-port 2382 -etcd-data-dir ./default.etcd-2 -log-level info &
	./bin/sentinel -id sentinel-3 -address :50054 -enable-embedded-etcd=true -etcd-port 2383 -etcd-peer-port 2384 -etcd-data-dir ./default.etcd-3 -log-level info &

# 测试内嵌etcd集成
test-embedded-etcd:
	./bin/sentinel -id sentinel-1 -address :50052 -enable-embedded-etcd=true -etcd-port 2379 -etcd-peer-port 2380 -etcd-data-dir ./test.etcd -log-level info &
	sleep 5
	go run test_etcd_integration.go
	pkill -f "bin/sentinel" || true 