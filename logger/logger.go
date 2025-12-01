package logger

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
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogMessage 日志消息结构
type LogMessage struct {
	Level     LogLevel
	Message   string
	Error     error
	Timestamp time.Time
	Category  string
}

// AppLoggerConfig 应用日志配置
type AppLoggerConfig struct {
	LogDir        string   // 日志目录
	LogLevel      LogLevel // 日志级别
	EnableConsole bool     // 是否输出到控制台
	EnableFile    bool     // 是否输出到文件
	MaxAge        int      // 日志保留天数
}

// DefaultAppLoggerConfig 默认应用日志配置
func DefaultAppLoggerConfig() *AppLoggerConfig {
	return &AppLoggerConfig{
		LogDir:        "./logs",
		LogLevel:      LogLevelInfo,
		EnableConsole: true,
		EnableFile:    true,
		MaxAge:        30,
	}
}

// AppLogger 应用日志记录器
type AppLogger struct {
	config        *AppLoggerConfig
	fileLogger    *log.Logger
	consoleLogger *log.Logger
	currentFile   *os.File
	currentDate   string
	mu            sync.Mutex
}

var (
	globalAppLogger *AppLogger
	appLoggerOnce   sync.Once
)

// InitAppLogger 初始化全局应用日志
func InitAppLogger(config *AppLoggerConfig) error {
	var initErr error
	appLoggerOnce = sync.Once{}
	appLoggerOnce.Do(func() {
		if config == nil {
			config = DefaultAppLoggerConfig()
		}

		logger := &AppLogger{
			config: config,
		}

		if config.EnableConsole {
			logger.consoleLogger = log.New(os.Stdout, "[APP] ", 0)
		}

		if config.EnableFile {
			if err := logger.initFileLogger(); err != nil {
				initErr = err
				return
			}
		}

		globalAppLogger = logger
		go logger.rotateCheck()
	})

	return initErr
}

// GetAppLogger 获取全局应用日志
func GetAppLogger() *AppLogger {
	if globalAppLogger == nil {
		InitAppLogger(nil)
	}
	return globalAppLogger
}

func (l *AppLogger) initFileLogger() error {
	if err := os.MkdirAll(l.config.LogDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	l.currentDate = time.Now().Format("2006-01-02")
	logFileName := filepath.Join(l.config.LogDir, fmt.Sprintf("app_%s.log", l.currentDate))

	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	l.currentFile = file
	l.fileLogger = log.New(file, "", 0)

	return nil
}

func (l *AppLogger) rotateCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		currentDate := time.Now().Format("2006-01-02")
		if currentDate != l.currentDate && l.config.EnableFile {
			l.rotateFile()
		}
		l.mu.Unlock()
	}
}

func (l *AppLogger) rotateFile() {
	if l.currentFile != nil {
		l.currentFile.Close()
	}

	l.currentDate = time.Now().Format("2006-01-02")
	logFileName := filepath.Join(l.config.LogDir, fmt.Sprintf("app_%s.log", l.currentDate))

	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("日志轮转失败: %v", err)
		return
	}

	l.currentFile = file
	l.fileLogger = log.New(file, "", 0)
	l.cleanOldLogs()
}

func (l *AppLogger) cleanOldLogs() {
	cutoff := time.Now().AddDate(0, 0, -l.config.MaxAge)

	entries, err := os.ReadDir(l.config.LogDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(l.config.LogDir, entry.Name()))
		}
	}
}

func (l *AppLogger) formatLog(msg *LogMessage) string {
	timestamp := msg.Timestamp.Format("2006-01-02 15:04:05.000")
	logLine := fmt.Sprintf("[%s] [%s]", timestamp, msg.Level.String())

	if msg.Category != "" {
		logLine += fmt.Sprintf(" [%s]", msg.Category)
	}

	logLine += fmt.Sprintf(" %s", msg.Message)

	if msg.Error != nil {
		logLine += fmt.Sprintf(" Error: %v", msg.Error)
	}

	return logLine
}

// Log 记录日志
func (l *AppLogger) Log(msg *LogMessage) {
	if msg.Level < l.config.LogLevel {
		return
	}

	logLine := l.formatLog(msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.config.EnableConsole && l.consoleLogger != nil {
		l.consoleLogger.Println(logLine)
	}

	if l.config.EnableFile && l.fileLogger != nil {
		l.fileLogger.Println(logLine)
	}
}

// Debug 调试日志
func (l *AppLogger) Debug(category, message string) {
	l.Log(&LogMessage{
		Level:     LogLevelDebug,
		Category:  category,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// Info 信息日志
func (l *AppLogger) Info(category, message string) {
	l.Log(&LogMessage{
		Level:     LogLevelInfo,
		Category:  category,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// Warn 警告日志
func (l *AppLogger) Warn(category, message string) {
	l.Log(&LogMessage{
		Level:     LogLevelWarn,
		Category:  category,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// Error 错误日志
func (l *AppLogger) Error(category, message string, err error) {
	l.Log(&LogMessage{
		Level:     LogLevelError,
		Category:  category,
		Message:   message,
		Error:     err,
		Timestamp: time.Now(),
	})
}

// Close 关闭日志
func (l *AppLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentFile != nil {
		return l.currentFile.Close()
	}
	return nil
}

// SetOutput 设置额外输出
func (l *AppLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.consoleLogger != nil {
		l.consoleLogger.SetOutput(io.MultiWriter(os.Stdout, w))
	}
}

// ========== 全局便捷函数 ==========

// Debug 全局调试日志
func Debug(category, message string) {
	GetAppLogger().Debug(category, message)
}

// Info 全局信息日志
func Info(category, message string) {
	GetAppLogger().Info(category, message)
}

// Warn 全局警告日志
func Warn(category, message string) {
	GetAppLogger().Warn(category, message)
}

// Error 全局错误日志
func Error(category, message string, err error) {
	GetAppLogger().Error(category, message, err)
}

// Logger 兼容旧接口
func Logger(e *error, message string) {
	if e != nil && *e != nil {
		GetAppLogger().Error("SYSTEM", message, *e)
	} else {
		GetAppLogger().Info("SYSTEM", message)
	}
}
