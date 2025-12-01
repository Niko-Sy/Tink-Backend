package middleware

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/logger"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 上下文键
const (
	DBKey      = "db"
	QueriesKey = "queries"
	TxKey      = "tx"
)

// DBConfig 数据库配置
type DBConfig struct {
	Driver          string        // 数据库驱动 (mysql, postgres, sqlite3)
	DSN             string        // 数据库连接字符串
	MaxOpenConns    int           // 最大打开连接数
	MaxIdleConns    int           // 最大空闲连接数
	ConnMaxLifetime time.Duration // 连接最大生命周期
	ConnMaxIdleTime time.Duration // 连接最大空闲时间
}

// DefaultDBConfig 返回默认数据库配置
func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		Driver:          "postgres",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// DBManager 数据库管理器
type DBManager struct {
	db          *sql.DB
	queries     *sqlcdb.Queries
	config      *DBConfig
	mu          sync.RWMutex
	closed      bool
	enableDBLog bool
}

// NewDBManager 创建新的数据库管理器
func NewDBManager(config *DBConfig) (*DBManager, error) {
	if config == nil {
		return nil, errors.New("数据库配置不能为空")
	}
	if config.DSN == "" {
		return nil, errors.New("数据库连接字符串不能为空")
	}

	db, err := sql.Open(config.Driver, config.DSN)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 初始化数据库日志
	if err := logger.InitDBLogger(nil); err != nil {
		log.Printf("初始化数据库日志失败: %v", err)
	}

	// 使用带日志的 DBTX 创建 queries
	loggedDB := logger.NewLoggedDBTX(db)
	queries := sqlcdb.New(loggedDB)

	manager := &DBManager{
		db:          db,
		queries:     queries,
		config:      config,
		enableDBLog: true,
	}

	log.Printf("数据库连接成功: %s", config.Driver)
	logger.LogDBInfo("CONNECT", fmt.Sprintf("数据库连接成功: %s", config.Driver))

	return manager, nil
}

// GetDB 获取原始数据库连接
func (m *DBManager) GetDB() *sql.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// GetQueries 获取查询对象
func (m *DBManager) GetQueries() *sqlcdb.Queries {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queries
}

// Close 关闭数据库连接
func (m *DBManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// 关闭数据库日志
	if dbLogger := logger.GetDBLogger(); dbLogger != nil {
		dbLogger.Close()
	}

	if err := m.db.Close(); err != nil {
		return fmt.Errorf("关闭数据库连接失败: %w", err)
	}

	log.Println("数据库连接已关闭")
	return nil
}

// Ping 检查数据库连接状态
func (m *DBManager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return errors.New("数据库连接已关闭")
	}
	return m.db.PingContext(ctx)
}

// Stats 获取数据库连接池统计信息
func (m *DBManager) Stats() sql.DBStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db.Stats()
}

// DBMiddleware 数据库中间件 - 将查询对象注入到 Gin 上下文
func DBMiddleware(manager *DBManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 将数据库管理器和查询对象添加到上下文
		c.Set(DBKey, manager.GetDB())
		c.Set(QueriesKey, manager.GetQueries())
		c.Next()
	}
}

// TransactionMiddleware 事务中间件 - 自动管理事务
func TransactionMiddleware(manager *DBManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始事务
		logger.LogDBInfo("TX_BEGIN", "开始事务")
		tx, err := manager.GetDB().BeginTx(c.Request.Context(), nil)
		if err != nil {
			logger.LogDBError("TX_BEGIN", "开始事务失败", err)
			c.AbortWithStatusJSON(500, gin.H{
				"code":    500,
				"message": "开始事务失败",
				"error":   err.Error(),
			})
			return
		}

		// 将事务和带事务的查询对象注入上下文
		c.Set(TxKey, tx)
		// 使用带日志的事务包装
		loggedTx := logger.NewLoggedDBTX(tx)
		c.Set(QueriesKey, sqlcdb.New(loggedTx))

		// 使用 defer 确保事务正确处理
		defer func() {
			if r := recover(); r != nil {
				logger.LogDBError("TX_ROLLBACK", "事务因 panic 回滚", fmt.Errorf("%v", r))
				tx.Rollback()
				panic(r)
			}
		}()

		c.Next()

		// 根据响应状态决定提交或回滚
		if c.Writer.Status() >= 400 || len(c.Errors) > 0 {
			if err := tx.Rollback(); err != nil {
				logger.LogDBError("TX_ROLLBACK", "事务回滚失败", err)
				log.Printf("事务回滚失败: %v", err)
			} else {
				logger.LogDBInfo("TX_ROLLBACK", "事务已回滚")
			}
		} else {
			if err := tx.Commit(); err != nil {
				logger.LogDBError("TX_COMMIT", "事务提交失败", err)
				log.Printf("事务提交失败: %v", err)
				c.AbortWithStatusJSON(500, gin.H{
					"code":    500,
					"message": "事务提交失败",
					"error":   err.Error(),
				})
			} else {
				logger.LogDBInfo("TX_COMMIT", "事务已提交")
			}
		}
	}
}

// HealthCheckMiddleware 数据库健康检查中间件
func HealthCheckMiddleware(manager *DBManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := manager.Ping(ctx); err != nil {
			c.AbortWithStatusJSON(503, gin.H{
				"code":    503,
				"message": "数据库服务不可用",
				"error":   err.Error(),
			})
			return
		}
		c.Next()
	}
}

// GetQueriesFromContext 从上下文获取查询对象
func GetQueriesFromContext(c *gin.Context) (*sqlcdb.Queries, error) {
	queries, exists := c.Get(QueriesKey)
	if !exists {
		return nil, errors.New("查询对象不存在于上下文中")
	}

	q, ok := queries.(*sqlcdb.Queries)
	if !ok {
		return nil, errors.New("查询对象类型错误")
	}

	return q, nil
}

// GetDBFromContext 从上下文获取数据库连接
func GetDBFromContext(c *gin.Context) (*sql.DB, error) {
	db, exists := c.Get(DBKey)
	if !exists {
		return nil, errors.New("数据库连接不存在于上下文中")
	}

	d, ok := db.(*sql.DB)
	if !ok {
		return nil, errors.New("数据库连接类型错误")
	}

	return d, nil
}

// GetTxFromContext 从上下文获取事务
func GetTxFromContext(c *gin.Context) (*sql.Tx, error) {
	tx, exists := c.Get(TxKey)
	if !exists {
		return nil, errors.New("事务不存在于上下文中")
	}

	t, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("事务类型错误")
	}

	return t, nil
}

// WithTransaction 在事务中执行函数
func WithTransaction(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	logger.LogDBInfo("TX_BEGIN", "开始事务")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.LogDBError("TX_BEGIN", "开始事务失败", err)
		return fmt.Errorf("开始事务失败: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			logger.LogDBError("TX_ROLLBACK", "事务因 panic 回滚", fmt.Errorf("%v", p))
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.LogDBError("TX_ROLLBACK", "事务回滚失败", rbErr)
			return fmt.Errorf("执行失败: %v, 回滚失败: %w", err, rbErr)
		}
		logger.LogDBInfo("TX_ROLLBACK", "事务已回滚")
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.LogDBError("TX_COMMIT", "事务提交失败", err)
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.LogDBInfo("TX_COMMIT", "事务已提交")
	return nil
}

// WithTransactionOptions 带选项在事务中执行函数
func WithTransactionOptions(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	logger.LogDBInfo("TX_BEGIN", fmt.Sprintf("开始事务 (Options: %+v)", opts))
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		logger.LogDBError("TX_BEGIN", "开始事务失败", err)
		return fmt.Errorf("开始事务失败: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			logger.LogDBError("TX_ROLLBACK", "事务因 panic 回滚", fmt.Errorf("%v", p))
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.LogDBError("TX_ROLLBACK", "事务回滚失败", rbErr)
			return fmt.Errorf("执行失败: %v, 回滚失败: %w", err, rbErr)
		}
		logger.LogDBInfo("TX_ROLLBACK", "事务已回滚")
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.LogDBError("TX_COMMIT", "事务提交失败", err)
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.LogDBInfo("TX_COMMIT", "事务已提交")
	return nil
}

// RetryableTransaction 可重试的事务执行
func RetryableTransaction(ctx context.Context, db *sql.DB, maxRetries int, fn func(tx *sql.Tx) error) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := WithTransaction(ctx, db, fn)
		if err == nil {
			return nil
		}

		lastErr = err

		// 检查是否是可重试的错误 (如死锁)
		if !isRetryableError(err) {
			return err
		}

		// 等待一段时间后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * 100 * time.Millisecond):
		}
	}

	return fmt.Errorf("事务在 %d 次重试后仍然失败: %w", maxRetries, lastErr)
}

// isRetryableError 检查是否是可重试的错误
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// 这里可以根据具体数据库添加更多可重试错误的判断
	// 例如 PostgreSQL 的死锁错误码等
	errStr := err.Error()
	return containsIgnoreCase(errStr, "deadlock") ||
		containsIgnoreCase(errStr, "lock wait timeout") ||
		containsIgnoreCase(errStr, "serialization failure")
}

// containsIgnoreCase 检查字符串是否包含子字符串 (不区分大小写)
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchIgnoreCase(a, b string) bool {
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ConnectionInfo 数据库连接信息
type ConnectionInfo struct {
	MaxOpenConnections int   `json:"max_open_connections"`
	OpenConnections    int   `json:"open_connections"`
	InUse              int   `json:"in_use"`
	Idle               int   `json:"idle"`
	WaitCount          int64 `json:"wait_count"`
	WaitDuration       int64 `json:"wait_duration_ms"`
	MaxIdleClosed      int64 `json:"max_idle_closed"`
	MaxLifetimeClosed  int64 `json:"max_lifetime_closed"`
}

// GetConnectionInfo 获取数据库连接信息
func (m *DBManager) GetConnectionInfo() ConnectionInfo {
	stats := m.Stats()
	return ConnectionInfo{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration.Milliseconds(),
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}
}

// DBStatusHandler 数据库状态处理器
func DBStatusHandler(manager *DBManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		status := "healthy"
		if err := manager.Ping(ctx); err != nil {
			status = "unhealthy"
		}

		c.JSON(200, gin.H{
			"status":     status,
			"connection": manager.GetConnectionInfo(),
		})
	}
}
