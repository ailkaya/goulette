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

// Fragment 消息片段
type Fragment struct {
	ID           uint64
	Topic        string
	Path         string
	Size         int64
	MessageCount int64
	CreatedAt    time.Time
	mu           sync.RWMutex
	file         *os.File
}

// FragmentHeader Fragment文件头
type FragmentHeader struct {
	Magic       [4]byte // "FRAG"
	Version     uint32
	TopicLength uint32
	Topic       []byte
	CreatedAt   int64
}

// MessageEntry 消息条目
type MessageEntry struct {
	MessageID   uint64
	Timestamp   int64
	PayloadSize uint32
	Payload     []byte
}

// FragmentManager Fragment管理器
type FragmentManager struct {
	baseDir       string
	fragments     map[string][]*Fragment // topic -> fragments
	mu            sync.RWMutex
	maxSize       int64
	maxMessages   int64
	ttl           time.Duration // TTL时间，0表示不设置TTL
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewFragmentManager 创建Fragment管理器
func NewFragmentManager(baseDir string, maxSize, maxMessages int64, ttl time.Duration) *FragmentManager {
	fm := &FragmentManager{
		baseDir:     baseDir,
		fragments:   make(map[string][]*Fragment),
		maxSize:     maxSize,
		maxMessages: maxMessages,
		ttl:         ttl,
		stopCleanup: make(chan struct{}),
	}

	// 如果设置了TTL，启动定期清理任务
	if ttl > 0 {
		fm.startCleanupTask()
	}

	return fm
}

// startCleanupTask 启动定期清理任务
func (fm *FragmentManager) startCleanupTask() {
	// 每小时检查一次过期fragment
	fm.cleanupTicker = time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-fm.cleanupTicker.C:
				fm.cleanupExpiredFragments()
			case <-fm.stopCleanup:
				return
			}
		}
	}()
}

// cleanupExpiredFragments 清理过期的fragment
func (fm *FragmentManager) cleanupExpiredFragments() {
	if fm.ttl <= 0 {
		return
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for topic, fragments := range fm.fragments {
		var validFragments []*Fragment

		for _, fragment := range fragments {
			// 检查fragment是否过期
			if now.Sub(fragment.CreatedAt) > fm.ttl {
				// 关闭文件
				if fragment.file != nil {
					fragment.file.Close()
				}

				// 删除文件
				if err := os.Remove(fragment.Path); err != nil {
					utils.Errorf("删除过期fragment文件失败: %v, path: %s", err, fragment.Path)
				} else {
					utils.Infof("删除过期fragment: topic=%s, id=%d, path=%s", topic, fragment.ID, fragment.Path)
					expiredCount++
				}
			} else {
				validFragments = append(validFragments, fragment)
			}
		}

		// 更新fragments列表
		if len(validFragments) == 0 {
			delete(fm.fragments, topic)
		} else {
			fm.fragments[topic] = validFragments
		}
	}

	if expiredCount > 0 {
		utils.Infof("清理完成，删除了 %d 个过期fragment", expiredCount)
	}
}

// CleanupExpiredFragments 手动清理过期的fragment（公开方法）
func (fm *FragmentManager) CleanupExpiredFragments() {
	fm.cleanupExpiredFragments()
}

// CreateFragment 创建新的Fragment
func (fm *FragmentManager) CreateFragment(topic string) (*Fragment, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 确保目录存在
	topicDir := filepath.Join(fm.baseDir, topic)
	if err := os.MkdirAll(topicDir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成Fragment ID
	fragments := fm.fragments[topic]
	fragmentID := uint64(len(fragments))

	// 创建Fragment文件
	filename := fmt.Sprintf("fragment_%d.frag", fragmentID)
	filepath := filepath.Join(topicDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("创建Fragment文件失败: %w", err)
	}

	// 写入文件头
	header := FragmentHeader{
		Magic:       [4]byte{'F', 'R', 'A', 'G'},
		Version:     1,
		TopicLength: uint32(len(topic)),
		Topic:       []byte(topic),
		CreatedAt:   time.Now().Unix(),
	}

	if err := binary.Write(file, binary.BigEndian, header.Magic); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入魔数失败: %w", err)
	}
	if err := binary.Write(file, binary.BigEndian, header.Version); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入版本失败: %w", err)
	}
	if err := binary.Write(file, binary.BigEndian, header.TopicLength); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入Topic长度失败: %w", err)
	}
	if _, err := file.Write(header.Topic); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入Topic失败: %w", err)
	}
	if err := binary.Write(file, binary.BigEndian, header.CreatedAt); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入创建时间失败: %w", err)
	}

	fragment := &Fragment{
		ID:        fragmentID,
		Topic:     topic,
		Path:      filepath,
		CreatedAt: time.Now(),
		file:      file,
	}

	fm.fragments[topic] = append(fm.fragments[topic], fragment)
	utils.Infof("创建Fragment: topic=%s, id=%d, path=%s", topic, fragmentID, filepath)

	return fragment, nil
}

// GetCurrentFragment 获取当前活跃的Fragment
func (fm *FragmentManager) GetCurrentFragment(topic string) (*Fragment, error) {
	fm.mu.RLock()
	fragments := fm.fragments[topic]
	fm.mu.RUnlock()

	if len(fragments) == 0 {
		return fm.CreateFragment(topic)
	}

	current := fragments[len(fragments)-1]

	// 检查是否需要创建新的Fragment
	if current.Size >= fm.maxSize || current.MessageCount >= fm.maxMessages {
		return fm.CreateFragment(topic)
	}

	return current, nil
}

// WriteMessage 写入消息到Fragment
func (fm *FragmentManager) WriteMessage(topic string, messageID uint64, payload []byte) error {
	fragment, err := fm.GetCurrentFragment(topic)
	if err != nil {
		return err
	}

	fragment.mu.Lock()
	defer fragment.mu.Unlock()

	// 写入消息条目
	entry := MessageEntry{
		MessageID:   messageID,
		Timestamp:   time.Now().Unix(),
		PayloadSize: uint32(len(payload)),
		Payload:     payload,
	}

	// 写入消息ID
	if err := binary.Write(fragment.file, binary.BigEndian, entry.MessageID); err != nil {
		return fmt.Errorf("写入消息ID失败: %w", err)
	}

	// 写入时间戳
	if err := binary.Write(fragment.file, binary.BigEndian, entry.Timestamp); err != nil {
		return fmt.Errorf("写入时间戳失败: %w", err)
	}

	// 写入负载大小
	if err := binary.Write(fragment.file, binary.BigEndian, entry.PayloadSize); err != nil {
		return fmt.Errorf("写入负载大小失败: %w", err)
	}

	// 写入负载数据
	if _, err := fragment.file.Write(entry.Payload); err != nil {
		return fmt.Errorf("写入负载数据失败: %w", err)
	}

	// 更新统计信息
	fragment.Size += int64(8 + 8 + 4 + len(payload)) // messageID + timestamp + payloadSize + payload
	fragment.MessageCount++

	// 强制刷盘
	if err := fragment.file.Sync(); err != nil {
		return fmt.Errorf("刷盘失败: %w", err)
	}

	return nil
}

// ReadMessages 从Fragment读取消息
func (fm *FragmentManager) ReadMessages(topic string, offset uint64, limit int) ([]MessageEntry, error) {
	fm.mu.RLock()
	fragments := fm.fragments[topic]
	fm.mu.RUnlock()

	if len(fragments) == 0 {
		return []MessageEntry{}, nil
	}

	var messages []MessageEntry
	var currentOffset uint64

	// 遍历所有Fragment
	for _, fragment := range fragments {
		if currentOffset >= offset+uint64(limit) {
			break
		}

		fragment.mu.RLock()
		file, err := os.Open(fragment.Path)
		if err != nil {
			fragment.mu.RUnlock()
			continue
		}

		// 跳过文件头
		headerSize := 4 + 4 + 4 + len(fragment.Topic) + 8 // magic + version + topicLength + topic + createdAt
		if _, err := file.Seek(int64(headerSize), io.SeekStart); err != nil {
			file.Close()
			fragment.mu.RUnlock()
			continue
		}

		// 读取消息
		for {
			if currentOffset >= offset+uint64(limit) {
				break
			}

			var messageID uint64
			if err := binary.Read(file, binary.BigEndian, &messageID); err != nil {
				if err == io.EOF {
					break
				}
				break
			}

			var timestamp int64
			if err := binary.Read(file, binary.BigEndian, &timestamp); err != nil {
				break
			}

			var payloadSize uint32
			if err := binary.Read(file, binary.BigEndian, &payloadSize); err != nil {
				break
			}

			payload := make([]byte, payloadSize)
			if _, err := io.ReadFull(file, payload); err != nil {
				break
			}

			if currentOffset >= offset {
				messages = append(messages, MessageEntry{
					MessageID:   messageID,
					Timestamp:   timestamp,
					PayloadSize: payloadSize,
					Payload:     payload,
				})
			}

			currentOffset++
		}

		file.Close()
		fragment.mu.RUnlock()
	}

	return messages, nil
}

// CloseTopic 关闭指定Topic的所有Fragment
func (fm *FragmentManager) CloseTopic(topic string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fragments, exists := fm.fragments[topic]
	if !exists {
		return nil // Topic不存在，无需关闭
	}

	// 关闭所有Fragment文件
	for _, fragment := range fragments {
		if fragment.file != nil {
			if err := fragment.file.Close(); err != nil {
				utils.Errorf("关闭Fragment文件失败: %v", err)
			}
		}
	}

	// 从映射中移除
	delete(fm.fragments, topic)

	utils.Infof("已关闭Topic的所有Fragment: %s", topic)
	return nil
}

// Close 关闭Fragment管理器
func (fm *FragmentManager) Close() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 停止清理任务
	if fm.cleanupTicker != nil {
		fm.cleanupTicker.Stop()
		close(fm.stopCleanup)
	}

	// 关闭所有Fragment文件
	for _, fragments := range fm.fragments {
		for _, fragment := range fragments {
			if fragment.file != nil {
				fragment.file.Close()
			}
		}
	}

	return nil
}

// GetFragments 获取指定 topic 的所有 fragment
func (fm *FragmentManager) GetFragments(topic string) []*Fragment {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return append([]*Fragment(nil), fm.fragments[topic]...)
}

// BaseDir 返回 fragment 存储的根目录
func (fm *FragmentManager) BaseDir() string {
	return fm.baseDir
}

// ReplicateFragmentToFile 将接收到的Fragment分片写入文件
func (fm *FragmentManager) ReplicateFragmentToFile(topic string, fragmentID uint64, data []byte, targetPath string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), "fragment_tmp_*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 写入数据
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("写入Fragment数据失败: %w", err)
	}

	// 确保所有数据都写入磁盘
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("同步临时文件失败: %w", err)
	}

	// 关闭临时文件
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	// 将临时文件重命名为目标文件
	if err := os.Rename(tmpFile.Name(), targetPath); err != nil {
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}

	return nil
}

// ReadFragmentFile 读取Fragment文件内容
func (fm *FragmentManager) ReadFragmentFile(topic string, fragmentID uint64) ([]byte, error) {
	fragmentPath := filepath.Join(fm.baseDir, topic, fmt.Sprintf("fragment_%d.frag", fragmentID))

	// 打开Fragment文件
	file, err := os.Open(fragmentPath)
	if err != nil {
		return nil, fmt.Errorf("打开Fragment文件失败: %w", err)
	}
	defer file.Close()

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取Fragment文件失败: %w", err)
	}

	return data, nil
}

// GetFragmentPath 获取Fragment文件的完整路径
func (fm *FragmentManager) GetFragmentPath(topic string, fragmentID uint64) string {
	return filepath.Join(fm.baseDir, topic, fmt.Sprintf("fragment_%d.frag", fragmentID))
}

// SelectFragmentsForReplication 根据条件选择需要复制的Fragments
func (fm *FragmentManager) SelectFragmentsForReplication(topic string, idx uint64, n uint64) []*Fragment {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var selectedFragments []*Fragment
	fragments := fm.fragments[topic]

	for _, fragment := range fragments {
		// 检查fragment是否符合条件：下标%n==idx
		if fragment.ID%n == idx {
			selectedFragments = append(selectedFragments, fragment)
		}
	}

	return selectedFragments
}

// IsFragmentFullyConsumed 检查Fragment是否已经被完全消费
// TODO: 实现具体的消费状态检查逻辑
func (fm *FragmentManager) IsFragmentFullyConsumed(topic string, fragmentID uint64) bool {
	// 这里需要实现检查Fragment是否已经被完全消费的逻辑
	// 可能需要与消费者组或消费位置管理器交互
	return false
}
