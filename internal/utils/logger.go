package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	logger      *log.Logger
	logFile     *os.File
	logLevel    LogLevel = INFO
	mu          sync.RWMutex
	initialized bool
)

// 日志级别名称映射
var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// InitLogger 初始化日志系统
func InitLogger(level string, logPath string) error {
	initFunc := func() error {
		mu.Lock()
		defer mu.Unlock()

		if initialized {
			return nil
		}

		// 设置日志级别
		switch level {
		case "debug":
			logLevel = DEBUG
		case "info":
			logLevel = INFO
		case "warn":
			logLevel = WARN
		case "error":
			logLevel = ERROR
		case "fatal":
			logLevel = FATAL
		default:
			logLevel = INFO
		}

		// 创建日志目录
		if logPath != "" {
			logDir := filepath.Dir(logPath)
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return fmt.Errorf("创建日志目录失败: %w", err)
			}

			// 打开日志文件
			file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("打开日志文件失败: %w", err)
			}

			logFile = file

			// 同时输出到文件和控制台
			multiWriter := io.MultiWriter(os.Stdout, logFile)
			logger = log.New(multiWriter, "", log.LstdFlags|log.Lshortfile)
		} else {
			// 只输出到控制台
			logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
		}
		initialized = true
		return nil
	}
	if err := initFunc(); err != nil {
		return err
	}
	Infof("日志系统初始化完成，级别: %s", levelNames[logLevel])
	return nil
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	mu.Lock()
	defer mu.Unlock()

	switch level {
	case "debug":
		logLevel = DEBUG
	case "info":
		logLevel = INFO
	case "warn":
		logLevel = WARN
	case "error":
		logLevel = ERROR
	case "fatal":
		logLevel = FATAL
	}
}

// Close 关闭日志文件
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	initialized = false
}

// shouldLog 检查是否应该记录该级别的日志
func shouldLog(level LogLevel) bool {
	mu.RLock()
	defer mu.RUnlock()
	return level >= logLevel
}

// formatMessage 格式化日志消息
func formatMessage(level LogLevel, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]

	if len(args) > 0 {
		return fmt.Sprintf("[%s] [%s] %s", timestamp, levelName, fmt.Sprintf(format, args...))
	}
	return fmt.Sprintf("[%s] [%s] %s", timestamp, levelName, format)
}

// 日志包装方法
func Debugf(format string, args ...interface{}) {
	if shouldLog(DEBUG) {
		logger.Print(formatMessage(DEBUG, format, args...))
	}
}

func Infof(format string, args ...interface{}) {
	if shouldLog(INFO) {
		logger.Print(formatMessage(INFO, format, args...))
	}
}

func Warnf(format string, args ...interface{}) {
	if shouldLog(WARN) {
		logger.Print(formatMessage(WARN, format, args...))
	}
}

func Errorf(format string, args ...interface{}) {
	if shouldLog(ERROR) {
		logger.Print(formatMessage(ERROR, format, args...))
	}
}

func Fatalf(format string, args ...interface{}) {
	if shouldLog(FATAL) {
		logger.Print(formatMessage(FATAL, format, args...))
		os.Exit(1)
	}
}

// 简单消息方法
func Debug(msg string) {
	log.Println(msg)
	if shouldLog(DEBUG) {
		logger.Print(formatMessage(DEBUG, "%s", msg))
	}
}

func Info(msg string) {
	if shouldLog(INFO) {
		logger.Print(formatMessage(INFO, "%s", msg))
	}
}

func Warn(msg string) {
	if shouldLog(WARN) {
		logger.Print(formatMessage(WARN, "%s", msg))
	}
}

func Error(msg string) {
	if shouldLog(ERROR) {
		logger.Print(formatMessage(ERROR, "%s", msg))
	}
}

func Fatal(msg string) {
	if shouldLog(FATAL) {
		logger.Print(formatMessage(FATAL, "%s", msg))
		os.Exit(1)
	}
}

// 错误日志方法
func ErrorWithErr(err error, format string, args ...interface{}) {
	if shouldLog(ERROR) {
		message := formatMessage(ERROR, format, args...)
		if err != nil {
			message += fmt.Sprintf(" - 错误: %v", err)
		}
		logger.Print(message)
	}
}

// 获取日志文件路径
func GetLogFilePath() string {
	mu.RLock()
	defer mu.RUnlock()

	if logFile != nil {
		return logFile.Name()
	}
	return ""
}

// 检查日志系统是否已初始化
func IsInitialized() bool {
	mu.RLock()
	defer mu.RUnlock()
	return initialized
}
