package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log *Logger

// Logger Log *zap.SugaredLogger
type Logger struct {
	Log *zap.SugaredLogger
}

func InitLogger(logpath string) {
	var log Logger
	log.Init(logpath)
	Log = &log
}

// init 初始化日志配置
// 初始化Logger
func (l *Logger) Init(logPath string) {
	//判断日志路径是否存在，不存在则创建
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		os.Create(logPath)
	}
	// 创建一个lumberjack.Logger，用于日志文件的切割和归档
	w := zapcore.AddSync(&lumberjack.Logger{
		// 日志文件路径
		Filename: logPath,
		// 日志文件最大大小，超过后进行切割
		MaxSize: 1024, //MB
		// 日志文件最大保存天数
		MaxAge: 30,
		// 使用本地时间
		LocalTime: true,
	})

	// 创建一个zapcore.EncoderConfig，用于配置日志的编码方式
	config := zap.NewProductionEncoderConfig()
	// 使用ISO8601时间编码方式
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	// 创建一个zapcore.Core，用于将日志输出到指定的位置
	core := zapcore.NewCore(
		// 使用JSON编码方式
		zapcore.NewJSONEncoder(config),
		// 输出到指定的位置
		w,
		// 使用原子级别
		zap.NewAtomicLevel(),
	)

	// 创建一个zap.Logger，并添加调用者信息和调用者跳过级别
	l.Log = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
}

// Info 打印信息
func (l *Logger) Info(args ...interface{}) {
	l.Log.Info(args...)
}

// Infof 打印信息，附带template信息
func (l *Logger) Infof(template string, args ...interface{}) {
	l.Log.Infof(template, args...)
}

// Warn 打印警告
func (l *Logger) Warn(args ...interface{}) {
	l.Log.Warn(args...)
}

// Warnf 打印警告，附带template信息
func (l *Logger) Warnf(template string, args ...interface{}) {
	l.Log.Warnf(template, args...)
}

// Error 打印错误
func (l *Logger) Error(args ...interface{}) {
	l.Log.Error(args...)
}

// Errorf 打印错误，附带template信息
func (l *Logger) Errorf(template string, args ...interface{}) {
	l.Log.Errorf(template, args...)
}

// Panic 打印Panic信息
func (l *Logger) Panic(args ...interface{}) {
	l.Log.Panic(args...)
}

// Panicf 打印Panic信息，附带template信息
func (l *Logger) Panicf(template string, args ...interface{}) {
	l.Log.Panicf(template, args...)
}

// DPanic 打印Panic信息
func (l *Logger) DPanic(args ...interface{}) {
	l.Log.DPanic(args...)
}

// DPanicf 打印DPanic信息，附带template信息
func (l *Logger) DPanicf(template string, args ...interface{}) {
	l.Log.DPanicf(template, args...)
}
