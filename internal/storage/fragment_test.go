package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFragmentManagerTTL(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "fragment_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建FragmentManager，设置1秒TTL
	fm := NewFragmentManager(tempDir, 1024*1024, 1000, 1*time.Second)
	defer fm.Close()

	// 创建fragment
	fragment, err := fm.CreateFragment("test-topic")
	assert.NoError(t, err)
	assert.NotNil(t, fragment)

	// 写入一些消息
	err = fm.WriteMessage("test-topic", 1, []byte("test message 1"))
	assert.NoError(t, err)

	err = fm.WriteMessage("test-topic", 2, []byte("test message 2"))
	assert.NoError(t, err)

	// 验证fragment存在
	fragments := fm.fragments["test-topic"]
	assert.Len(t, fragments, 1)

	// 等待TTL过期
	time.Sleep(2 * time.Second)

	// 手动清理过期fragment
	fm.CleanupExpiredFragments()

	// 验证fragment已被删除
	fragments = fm.fragments["test-topic"]
	assert.Len(t, fragments, 0)

	// 验证文件已被删除
	_, err = os.Stat(fragment.Path)
	assert.True(t, os.IsNotExist(err))
}

func TestFragmentManagerNoTTL(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "fragment_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建FragmentManager，不设置TTL
	fm := NewFragmentManager(tempDir, 1024*1024, 1000, 0)
	defer fm.Close()

	// 创建fragment
	fragment, err := fm.CreateFragment("test-topic")
	assert.NoError(t, err)
	assert.NotNil(t, fragment)

	// 写入一些消息
	err = fm.WriteMessage("test-topic", 1, []byte("test message 1"))
	assert.NoError(t, err)

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 手动清理过期fragment
	fm.CleanupExpiredFragments()

	// 验证fragment仍然存在（因为没有TTL）
	fragments := fm.fragments["test-topic"]
	assert.Len(t, fragments, 1)

	// 验证文件仍然存在
	_, err = os.Stat(fragment.Path)
	assert.NoError(t, err)
}

func TestFragmentManagerCleanupTask(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "fragment_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建FragmentManager，设置1秒TTL
	fm := NewFragmentManager(tempDir, 1024*1024, 1000, 1*time.Second)

	// 创建fragment
	fragment, err := fm.CreateFragment("test-topic")
	assert.NoError(t, err)
	assert.NotNil(t, fragment)

	// 写入一些消息
	err = fm.WriteMessage("test-topic", 1, []byte("test message 1"))
	assert.NoError(t, err)

	// 等待TTL过期
	time.Sleep(2 * time.Second)

	// 关闭FragmentManager（这会触发清理任务停止）
	fm.Close()

	// 验证fragment已被删除
	fragments := fm.fragments["test-topic"]
	assert.Len(t, fragments, 0)

	// 验证文件已被删除
	_, err = os.Stat(fragment.Path)
	assert.True(t, os.IsNotExist(err))
}
