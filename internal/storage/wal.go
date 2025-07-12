package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ailkaya/goulette/internal/utils"
)

// WALEntry WAL条目
type WALEntry struct {
	Type      uint8
	Topic     string
	MessageID uint64
	Payload   []byte
	Timestamp int64
}

// WALHeader WAL文件头
type WALHeader struct {
	Magic   [4]byte // "WAL"
	Version uint32
}

// WAL WAL预写日志
type WAL struct {
	dir           string
	currentFile   *os.File
	currentPath   string
	mu            sync.Mutex
	maxFileSize   int64
	flushInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewWAL 创建新的WAL
func NewWAL(dir string, maxFileSize int64, flushInterval time.Duration) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建WAL目录失败: %w", err)
	}

	wal := &WAL{
		dir:           dir,
		maxFileSize:   maxFileSize,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}

	if err := wal.createNewFile(); err != nil {
		return nil, err
	}

	// 启动后台刷盘协程
	wal.wg.Add(1)
	go wal.flushRoutine()

	return wal, nil
}

// createNewFile 创建新的WAL文件
func (wal *WAL) createNewFile() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	// 关闭当前文件
	if wal.currentFile != nil {
		wal.currentFile.Close()
	}

	// 生成新文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("wal_%s.log", timestamp)
	filepath := filepath.Join(wal.dir, filename)

	// 创建新文件
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建WAL文件失败: %w", err)
	}

	// 写入文件头
	header := WALHeader{
		Magic:   [4]byte{'W', 'A', 'L', '1'},
		Version: 1,
	}

	if err := binary.Write(file, binary.BigEndian, header.Magic); err != nil {
		file.Close()
		return fmt.Errorf("写入WAL魔数失败: %w", err)
	}
	if err := binary.Write(file, binary.BigEndian, header.Version); err != nil {
		file.Close()
		return fmt.Errorf("写入WAL版本失败: %w", err)
	}

	wal.currentFile = file
	wal.currentPath = filepath

	utils.Infof("创建新WAL文件: %s", filepath)
	return nil
}

// Write 写入WAL条目
func (wal *WAL) Write(entry *WALEntry) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	// 检查文件大小
	if wal.currentFile != nil {
		if stat, err := wal.currentFile.Stat(); err == nil {
			if stat.Size() >= wal.maxFileSize {
				if err := wal.createNewFile(); err != nil {
					return err
				}
			}
		}
	}

	// 计算条目大小
	topicBytes := []byte(entry.Topic)
	entrySize := 1 + 4 + len(topicBytes) + 8 + 4 + len(entry.Payload) + 8 // type + topicLen + topic + messageID + payloadLen + payload + timestamp

	// 写入条目大小
	if err := binary.Write(wal.currentFile, binary.BigEndian, uint32(entrySize)); err != nil {
		return fmt.Errorf("写入条目大小失败: %w", err)
	}

	// 写入条目类型
	if err := binary.Write(wal.currentFile, binary.BigEndian, entry.Type); err != nil {
		return fmt.Errorf("写入条目类型失败: %w", err)
	}

	// 写入Topic长度和Topic
	if err := binary.Write(wal.currentFile, binary.BigEndian, uint32(len(topicBytes))); err != nil {
		return fmt.Errorf("写入Topic长度失败: %w", err)
	}
	if _, err := wal.currentFile.Write(topicBytes); err != nil {
		return fmt.Errorf("写入Topic失败: %w", err)
	}

	// 写入消息ID
	if err := binary.Write(wal.currentFile, binary.BigEndian, entry.MessageID); err != nil {
		return fmt.Errorf("写入消息ID失败: %w", err)
	}

	// 写入负载长度和负载
	if err := binary.Write(wal.currentFile, binary.BigEndian, uint32(len(entry.Payload))); err != nil {
		return fmt.Errorf("写入负载长度失败: %w", err)
	}
	if _, err := wal.currentFile.Write(entry.Payload); err != nil {
		return fmt.Errorf("写入负载失败: %w", err)
	}

	// 写入时间戳
	if err := binary.Write(wal.currentFile, binary.BigEndian, entry.Timestamp); err != nil {
		return fmt.Errorf("写入时间戳失败: %w", err)
	}

	return nil
}

// WriteMessage 写入消息到WAL
func (wal *WAL) WriteMessage(topic string, messageID uint64, payload []byte) error {
	entry := &WALEntry{
		Type:      1, // 消息类型
		Topic:     topic,
		MessageID: messageID,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	return wal.Write(entry)
}

// WriteRemoveTopic 写入移除Topic操作到WAL
func (wal *WAL) WriteRemoveTopic(topic string, reason string) error {
	entry := &WALEntry{
		Type:      2, // 移除Topic类型
		Topic:     topic,
		MessageID: 0,              // 移除操作不需要消息ID
		Payload:   []byte(reason), // 将原因作为payload存储
		Timestamp: time.Now().Unix(),
	}

	return wal.Write(entry)
}

// flushRoutine 后台刷盘协程
func (wal *WAL) flushRoutine() {
	defer wal.wg.Done()

	ticker := time.NewTicker(wal.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wal.Flush()
		case <-wal.stopChan:
			wal.Flush()
			return
		}
	}
}

// Flush 强制刷盘
func (wal *WAL) Flush() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.currentFile != nil {
		return wal.currentFile.Sync()
	}
	return nil
}

// Replay 重放WAL文件
func (wal *WAL) Replay(handler func(*WALEntry) error) error {
	// 读取WAL目录中的所有文件
	files, err := filepath.Glob(filepath.Join(wal.dir, "wal_*.log"))
	if err != nil {
		return fmt.Errorf("读取WAL文件列表失败: %w", err)
	}

	// 按文件名排序（时间顺序）
	// 这里简化处理，实际应该按时间戳排序

	for _, filepath := range files {
		if err := wal.replayFile(filepath, handler); err != nil {
			utils.Warnf("重放WAL文件失败: %s, error: %v", filepath, err)
			continue
		}
	}

	return nil
}

// replayFile 重放单个WAL文件
func (wal *WAL) replayFile(filepath string, handler func(*WALEntry) error) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("打开WAL文件失败: %w", err)
	}
	defer file.Close()

	// 读取文件头
	var magic [4]byte
	if err := binary.Read(file, binary.BigEndian, &magic); err != nil {
		return fmt.Errorf("读取WAL魔数失败: %w", err)
	}
	if magic != [4]byte{'W', 'A', 'L', '1'} {
		return fmt.Errorf("无效的WAL文件格式")
	}

	var version uint32
	if err := binary.Read(file, binary.BigEndian, &version); err != nil {
		return fmt.Errorf("读取WAL版本失败: %w", err)
	}

	// 读取条目
	for {
		var entrySize uint32
		if err := binary.Read(file, binary.BigEndian, &entrySize); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("读取条目大小失败: %w", err)
		}

		// 读取条目数据
		entryData := make([]byte, entrySize)
		if _, err := io.ReadFull(file, entryData); err != nil {
			return fmt.Errorf("读取条目数据失败: %w", err)
		}

		// 解析条目
		entry, err := wal.parseEntry(entryData)
		if err != nil {
			return fmt.Errorf("解析条目失败: %w", err)
		}

		// 调用处理器
		if err := handler(entry); err != nil {
			return fmt.Errorf("处理条目失败: %w", err)
		}
	}

	return nil
}

// parseEntry 解析WAL条目
func (wal *WAL) parseEntry(data []byte) (*WALEntry, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("条目数据太短")
	}

	offset := 0

	// 读取类型
	entryType := data[offset]
	offset++

	// 读取Topic长度和Topic
	if offset+4 > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取Topic长度")
	}
	topicLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	if offset+int(topicLen) > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取Topic")
	}
	topic := string(data[offset : offset+int(topicLen)])
	offset += int(topicLen)

	// 读取消息ID
	if offset+8 > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取消息ID")
	}
	messageID := binary.BigEndian.Uint64(data[offset:])
	offset += 8

	// 读取负载长度和负载
	if offset+4 > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取负载长度")
	}
	payloadLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	if offset+int(payloadLen) > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取负载")
	}
	payload := make([]byte, payloadLen)
	copy(payload, data[offset:offset+int(payloadLen)])
	offset += int(payloadLen)

	// 读取时间戳
	if offset+8 > len(data) {
		return nil, fmt.Errorf("数据不足，无法读取时间戳")
	}
	timestamp := binary.BigEndian.Uint64(data[offset:])

	return &WALEntry{
		Type:      entryType,
		Topic:     topic,
		MessageID: messageID,
		Payload:   payload,
		Timestamp: int64(timestamp),
	}, nil
}

// Close 关闭WAL
func (wal *WAL) Close() error {
	close(wal.stopChan)
	wal.wg.Wait()

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if wal.currentFile != nil {
		return wal.currentFile.Close()
	}
	return nil
}
