package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogger_FileOutput(t *testing.T) {
	// 清理之前的日志文件
	testLogPath := "test_logs/test.log"
	os.RemoveAll("test_logs")

	// 初始化日志系统
	err := InitLogger("debug", testLogPath)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
	defer Close()

	// 写入一些测试日志
	Info("这是一条信息日志")
	Warn("这是一条警告日志")
	Error("这是一条错误日志")
	Debug("这是一条调试日志")

	// 等待文件写入
	time.Sleep(100 * time.Millisecond)

	// 检查日志文件是否存在
	if _, err := os.Stat(testLogPath); os.IsNotExist(err) {
		t.Fatal("日志文件未创建")
	}

	// 读取日志文件内容
	content, err := os.ReadFile(testLogPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)

	// 验证日志内容
	if !strings.Contains(contentStr, "这是一条信息日志") {
		t.Error("信息日志未写入文件")
	}
	if !strings.Contains(contentStr, "这是一条警告日志") {
		t.Error("警告日志未写入文件")
	}
	if !strings.Contains(contentStr, "这是一条错误日志") {
		t.Error("错误日志未写入文件")
	}
	if !strings.Contains(contentStr, "这是一条调试日志") {
		t.Error("调试日志未写入文件")
	}

	// 验证日志格式
	if !strings.Contains(contentStr, "[INFO]") {
		t.Error("日志级别标记缺失")
	}
	if !strings.Contains(contentStr, "[WARN]") {
		t.Error("日志级别标记缺失")
	}
	if !strings.Contains(contentStr, "[ERROR]") {
		t.Error("日志级别标记缺失")
	}
	if !strings.Contains(contentStr, "[DEBUG]") {
		t.Error("日志级别标记缺失")
	}
}

func TestLogger_LogLevel(t *testing.T) {
	// 测试不同日志级别
	testCases := []struct {
		level       string
		expected    []string
		notExpected []string
	}{
		{
			level:       "debug",
			expected:    []string{"[DEBUG]", "[INFO]", "[WARN]", "[ERROR]"},
			notExpected: []string{},
		},
		{
			level:       "info",
			expected:    []string{"[INFO]", "[WARN]", "[ERROR]"},
			notExpected: []string{"[DEBUG]"},
		},
		{
			level:       "warn",
			expected:    []string{"[WARN]", "[ERROR]"},
			notExpected: []string{"[DEBUG]", "[INFO]"},
		},
		{
			level:       "error",
			expected:    []string{"[ERROR]"},
			notExpected: []string{"[DEBUG]", "[INFO]", "[WARN]"},
		},
	}

	testLogPath := "test_logs/level_test.log"
	for _, tc := range testCases {
		t.Run("level_"+tc.level, func(t *testing.T) {
			// 重新初始化日志系统
			Close()
			os.RemoveAll("test_logs/level_test.log")
			err := InitLogger(tc.level, testLogPath)
			if err != nil {
				t.Fatalf("初始化日志失败: %v", err)
			}

			// 写入所有级别的日志
			Debug("调试信息")
			Info("一般信息")
			Warn("警告信息")
			Error("错误信息")

			// 等待文件写入
			time.Sleep(100 * time.Millisecond)

			// 读取日志文件内容
			content, err := os.ReadFile(testLogPath)
			if err != nil {
				t.Fatalf("读取日志文件失败: %v", err)
			}

			contentStr := string(content)

			// 验证应该出现的日志级别
			for _, expected := range tc.expected {
				if !strings.Contains(contentStr, expected) {
					t.Errorf("日志级别 %s 应该出现但未找到", expected)
				}
			}

			// 验证不应该出现的日志级别
			for _, notExpected := range tc.notExpected {
				if strings.Contains(contentStr, notExpected) {
					t.Errorf("日志级别 %s 不应该出现但找到了", notExpected)
				}
			}
		})
	}
}

func TestLogger_FormatMessage(t *testing.T) {
	// 清理之前的日志文件
	testLogPath := "test_logs/format_test.log"
	os.RemoveAll("test_logs")

	// 初始化日志系统
	err := InitLogger("debug", testLogPath)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
	defer Close()

	// 测试格式化消息
	Infof("用户 %s 登录成功，ID: %d", "张三", 12345)
	Errorf("处理请求失败: %s", "连接超时")

	// 等待文件写入
	time.Sleep(100 * time.Millisecond)

	// 读取日志文件内容
	content, err := os.ReadFile(testLogPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)

	// 验证格式化消息
	if !strings.Contains(contentStr, "用户 张三 登录成功，ID: 12345") {
		t.Error("格式化消息未正确写入")
	}
	if !strings.Contains(contentStr, "处理请求失败: 连接超时") {
		t.Error("格式化消息未正确写入")
	}
}

func TestLogger_ErrorWithErr(t *testing.T) {
	// 清理之前的日志文件
	testLogPath := "test_logs/error_test.log"
	os.RemoveAll("test_logs")

	// 初始化日志系统
	err := InitLogger("error", testLogPath)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
	defer Close()

	// 测试错误日志
	testErr := fmt.Errorf("测试错误")
	ErrorWithErr(testErr, "操作失败")

	// 等待文件写入
	time.Sleep(100 * time.Millisecond)

	// 读取日志文件内容
	content, err := os.ReadFile(testLogPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	contentStr := string(content)

	// 验证错误日志
	if !strings.Contains(contentStr, "操作失败") {
		t.Error("错误消息未正确写入")
	}
	if !strings.Contains(contentStr, "测试错误") {
		t.Error("错误详情未正确写入")
	}
}

func TestLogger_ConsoleOnly(t *testing.T) {
	// 测试只输出到控制台
	Close()
	err := InitLogger("info", "")
	if err != nil {
		t.Fatalf("初始化控制台日志失败: %v", err)
	}
	defer Close()

	// 验证日志文件路径为空
	if GetLogFilePath() != "" {
		t.Error("控制台日志模式应该没有日志文件路径")
	}

	// 验证已初始化
	if !IsInitialized() {
		t.Error("日志系统应该已初始化")
	}

	// 写入日志（应该只输出到控制台）
	Info("控制台日志测试")
	Warn("控制台警告测试")
}

func TestLogger_DirectoryCreation(t *testing.T) {
	// 测试自动创建目录
	testLogPath := "test_logs/nested/dir/test.log"
	os.RemoveAll("test_logs")

	// 初始化日志系统
	err := InitLogger("info", testLogPath)
	if err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}
	defer Close()

	// 验证目录是否创建
	if _, err := os.Stat(filepath.Dir(testLogPath)); os.IsNotExist(err) {
		t.Error("日志目录未自动创建")
	}

	// 验证日志文件是否创建
	if _, err := os.Stat(testLogPath); os.IsNotExist(err) {
		t.Error("日志文件未创建")
	}
}
