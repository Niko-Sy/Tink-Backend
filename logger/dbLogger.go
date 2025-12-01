package logger

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DBLogLevel 日志级别
type DBLogLevel int

const (
	DBLogLevelDebug DBLogLevel = iota
	DBLogLevelInfo
	DBLogLevelWarn
	DBLogLevelError
)

func (l DBLogLevel) String() string {
	switch l {
	case DBLogLevelDebug:
		return "DEBUG"
	case DBLogLevelInfo:
		return "INFO"
	case DBLogLevelWarn:
		return "WARN"
	case DBLogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// DBLogEntry 数据库日志条目
type DBLogEntry struct {
	Timestamp    time.Time     `json:"timestamp"`
	Level        DBLogLevel    `json:"level"`
	Operation    string        `json:"operation"`
	Query        string        `json:"query,omitempty"`
	Args         []interface{} `json:"args,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	RowsAffected int64         `json:"rows_affected,omitempty"`
	Error        string        `json:"error,omitempty"`
	CallerInfo   string        `json:"caller,omitempty"`
}

// DBLoggerConfig 数据库日志配置
type DBLoggerConfig struct {
	LogDir         string        // 日志目录
	LogLevel       DBLogLevel    // 日志级别
	MaxFileSize    int64         // 单个日志文件最大大小（字节）
	MaxAge         int           // 日志保留天数
	RotateInterval time.Duration // 日志轮转间隔
	EnableConsole  bool          // 是否输出到控制台
	EnableFile     bool          // 是否输出到文件
	SlowQueryTime  time.Duration // 慢查询阈值
}

// DefaultDBLoggerConfig 默认日志配置
func DefaultDBLoggerConfig() *DBLoggerConfig {
	return &DBLoggerConfig{
		LogDir:         "./logs",
		LogLevel:       DBLogLevelInfo,
		MaxFileSize:    100 * 1024 * 1024, // 100MB
		MaxAge:         30,                // 30天
		RotateInterval: 24 * time.Hour,    // 每天轮转
		EnableConsole:  false,             // 不输出到控制台
		EnableFile:     true,
		SlowQueryTime:  200 * time.Millisecond,
	}
}

// DBLogger 数据库日志记录器
type DBLogger struct {
	config        *DBLoggerConfig
	fileLogger    *log.Logger
	consoleLogger *log.Logger
	currentFile   *os.File
	currentDate   string
	mu            sync.Mutex
}

// 全局数据库日志实例
var (
	globalDBLogger *DBLogger
	dbLoggerOnce   sync.Once
)

// InitDBLogger 初始化全局数据库日志记录器
func InitDBLogger(config *DBLoggerConfig) error {
	var initErr error
	dbLoggerOnce = sync.Once{} // 允许重新初始化
	dbLoggerOnce.Do(func() {
		if config == nil {
			config = DefaultDBLoggerConfig()
		}

		logger := &DBLogger{
			config: config,
		}

		// 初始化控制台日志
		if config.EnableConsole {
			logger.consoleLogger = log.New(os.Stdout, "[DB] ", 0)
		}

		// 初始化文件日志
		if config.EnableFile {
			if err := logger.initFileLogger(); err != nil {
				initErr = err
				return
			}
		}

		globalDBLogger = logger

		// 启动日志轮转检查
		go logger.rotateCheck()
	})

	return initErr
}

// GetDBLogger 获取全局数据库日志记录器
func GetDBLogger() *DBLogger {
	if globalDBLogger == nil {
		InitDBLogger(nil)
	}
	return globalDBLogger
}

// initFileLogger 初始化文件日志记录器
func (l *DBLogger) initFileLogger() error {
	// 确保日志目录存在
	if err := os.MkdirAll(l.config.LogDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件
	l.currentDate = time.Now().Format("2006-01-02")
	logFileName := filepath.Join(l.config.LogDir, fmt.Sprintf("db_%s.log", l.currentDate))

	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	l.currentFile = file
	l.fileLogger = log.New(file, "", 0)

	return nil
}

// rotateCheck 日志轮转检查
func (l *DBLogger) rotateCheck() {
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

// rotateFile 轮转日志文件
func (l *DBLogger) rotateFile() {
	if l.currentFile != nil {
		l.currentFile.Close()
	}

	l.currentDate = time.Now().Format("2006-01-02")
	logFileName := filepath.Join(l.config.LogDir, fmt.Sprintf("db_%s.log", l.currentDate))

	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("日志轮转失败: %v", err)
		return
	}

	l.currentFile = file
	l.fileLogger = log.New(file, "", 0)

	// 清理过期日志
	l.cleanOldLogs()
}

// cleanOldLogs 清理过期日志
func (l *DBLogger) cleanOldLogs() {
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

// formatLog 格式化日志
func (l *DBLogger) formatLog(entry *DBLogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05.000")

	logLine := fmt.Sprintf("[%s] [%s] [%s]",
		timestamp,
		entry.Level.String(),
		entry.Operation,
	)

	if entry.Query != "" {
		// 截断过长的查询
		query := entry.Query
		if len(query) > 500 {
			query = query[:500] + "..."
		}
		logLine += fmt.Sprintf(" SQL: %s", query)
	}

	if len(entry.Args) > 0 {
		logLine += fmt.Sprintf(" Args: %v", entry.Args)
	}

	if entry.Duration > 0 {
		logLine += fmt.Sprintf(" Duration: %v", entry.Duration)
	}

	if entry.RowsAffected >= 0 {
		logLine += fmt.Sprintf(" Rows: %d", entry.RowsAffected)
	}

	if entry.Error != "" {
		logLine += fmt.Sprintf(" Error: %s", entry.Error)
	}

	return logLine
}

// Log 记录日志
func (l *DBLogger) Log(entry *DBLogEntry) {
	if entry.Level < l.config.LogLevel {
		return
	}

	logLine := l.formatLog(entry)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.config.EnableConsole && l.consoleLogger != nil {
		l.consoleLogger.Println(logLine)
	}

	if l.config.EnableFile && l.fileLogger != nil {
		l.fileLogger.Println(logLine)
	}
}

// LogQuery 记录查询日志
func (l *DBLogger) LogQuery(operation, query string, args []interface{}, duration time.Duration, err error) {
	entry := &DBLogEntry{
		Timestamp:    time.Now(),
		Level:        DBLogLevelInfo,
		Operation:    operation,
		Query:        query,
		Args:         args,
		Duration:     duration,
		RowsAffected: -1,
	}

	// 慢查询标记
	if duration > l.config.SlowQueryTime {
		entry.Level = DBLogLevelWarn
		entry.Operation = "SLOW_" + operation
	}

	if err != nil {
		entry.Level = DBLogLevelError
		entry.Error = err.Error()
	}

	l.Log(entry)
}

// LogExec 记录执行日志
func (l *DBLogger) LogExec(operation, query string, args []interface{}, duration time.Duration, rowsAffected int64, err error) {
	entry := &DBLogEntry{
		Timestamp:    time.Now(),
		Level:        DBLogLevelInfo,
		Operation:    operation,
		Query:        query,
		Args:         args,
		Duration:     duration,
		RowsAffected: rowsAffected,
	}

	if duration > l.config.SlowQueryTime {
		entry.Level = DBLogLevelWarn
		entry.Operation = "SLOW_" + operation
	}

	if err != nil {
		entry.Level = DBLogLevelError
		entry.Error = err.Error()
	}

	l.Log(entry)
}

// Debug 记录调试日志
func (l *DBLogger) Debug(operation string, message string) {
	l.Log(&DBLogEntry{
		Timestamp: time.Now(),
		Level:     DBLogLevelDebug,
		Operation: operation,
		Query:     message,
	})
}

// Info 记录信息日志
func (l *DBLogger) Info(operation string, message string) {
	l.Log(&DBLogEntry{
		Timestamp: time.Now(),
		Level:     DBLogLevelInfo,
		Operation: operation,
		Query:     message,
	})
}

// Warn 记录警告日志
func (l *DBLogger) Warn(operation string, message string) {
	l.Log(&DBLogEntry{
		Timestamp: time.Now(),
		Level:     DBLogLevelWarn,
		Operation: operation,
		Query:     message,
	})
}

// Error 记录错误日志
func (l *DBLogger) Error(operation string, message string, err error) {
	entry := &DBLogEntry{
		Timestamp: time.Now(),
		Level:     DBLogLevelError,
		Operation: operation,
		Query:     message,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	l.Log(entry)
}

// Close 关闭日志记录器
func (l *DBLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentFile != nil {
		return l.currentFile.Close()
	}
	return nil
}

// ========== 带日志的 DBTX 包装器 ==========

// LoggedDBTX 带日志的数据库事务接口
type LoggedDBTX struct {
	db     DBTX
	logger *DBLogger
}

// DBTX 数据库事务接口
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// NewLoggedDBTX 创建带日志的 DBTX
func NewLoggedDBTX(db DBTX) *LoggedDBTX {
	return &LoggedDBTX{
		db:     db,
		logger: GetDBLogger(),
	}
}

// ExecContext 执行带日志的 SQL
func (l *LoggedDBTX) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := l.db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	var rowsAffected int64 = -1
	if result != nil {
		rowsAffected, _ = result.RowsAffected()
	}

	l.logger.LogExec("EXEC", query, args, duration, rowsAffected, err)
	return result, err
}

// PrepareContext 准备语句带日志
func (l *LoggedDBTX) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	start := time.Now()
	stmt, err := l.db.PrepareContext(ctx, query)
	duration := time.Since(start)

	l.logger.LogQuery("PREPARE", query, nil, duration, err)
	return stmt, err
}

// QueryContext 查询带日志
func (l *LoggedDBTX) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := l.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	l.logger.LogQuery("QUERY", query, args, duration, err)
	return rows, err
}

// QueryRowContext 查询单行带日志
func (l *LoggedDBTX) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := l.db.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	l.logger.LogQuery("QUERY_ROW", query, args, duration, nil)
	return row
}

// ========== 便捷函数 ==========

// LogDBQuery 记录数据库查询（全局函数）
func LogDBQuery(operation, query string, args []interface{}, duration time.Duration, err error) {
	GetDBLogger().LogQuery(operation, query, args, duration, err)
}

// LogDBExec 记录数据库执行（全局函数）
func LogDBExec(operation, query string, args []interface{}, duration time.Duration, rowsAffected int64, err error) {
	GetDBLogger().LogExec(operation, query, args, duration, rowsAffected, err)
}

// LogDBInfo 记录数据库信息日志（全局函数）
func LogDBInfo(operation, message string) {
	GetDBLogger().Info(operation, message)
}

// LogDBError 记录数据库错误日志（全局函数）
func LogDBError(operation, message string, err error) {
	GetDBLogger().Error(operation, message, err)
}

// SetDBLogOutput 设置额外的日志输出
func SetDBLogOutput(w io.Writer) {
	logger := GetDBLogger()
	logger.mu.Lock()
	defer logger.mu.Unlock()

	if logger.consoleLogger != nil {
		logger.consoleLogger.SetOutput(io.MultiWriter(os.Stdout, w))
	}
}
