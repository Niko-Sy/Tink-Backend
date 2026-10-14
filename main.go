package main

import (
	"chatroombackend/api/websocketmsg"
	"chatroombackend/config"
	"chatroombackend/logger"
	"chatroombackend/middleware"
	"chatroombackend/server"
	socialservice "chatroombackend/service/social"
	"chatroombackend/utils"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq" // PostgreSQL 驱动
)

var dbManager *middleware.DBManager
var appConfig *config.Config

func initialize(configPath string) {
	// 加载配置文件
	var err error
	appConfig, err = config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	if appConfig.JWT.Secret != "" {
		middleware.JWTSecretKey = []byte(appConfig.JWT.Secret)
	}
	if len(appConfig.JWT.Secret) < 32 || strings.Contains(appConfig.JWT.Secret, "your-secret-key") {
		log.Fatal("config.yaml 的 jwt.secret 必须为至少32字节的独立随机密钥")
	}
	middleware.JWTExpireSeconds = appConfig.JWT.Expiration
	middleware.JWTRefreshSeconds = appConfig.JWT.RefreshExpiration
	// 初始化应用日志
	loggerConfig := &logger.AppLoggerConfig{
		LogDir:        appConfig.Logger.LogDir,
		LogLevel:      parseLogLevel(appConfig.Logger.LogLevel),
		EnableConsole: appConfig.Logger.EnableConsole,
		EnableFile:    appConfig.Logger.EnableFile,
		MaxAge:        appConfig.Logger.MaxAge,
	}
	if err := logger.InitAppLogger(loggerConfig); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	// 初始化数据库配置
	dbConfig := middleware.DefaultDBConfig()
	dbConfig.DSN = appConfig.GetDatabaseURL()
	dbConfig.MaxOpenConns = appConfig.Database.MaxOpenConns
	dbConfig.MaxIdleConns = appConfig.Database.MaxIdleConns
	dbConfig.ConnMaxLifetime = time.Duration(appConfig.Database.ConnMaxLifetime) * time.Second

	dbManager, err = middleware.NewDBManager(dbConfig)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 初始化图片上传配置
	imageConfig := &utils.ImageConfig{
		UploadDir:     appConfig.Upload.UploadDir,
		MaxSize:       appConfig.Upload.MaxSize,
		AllowedTypes:  appConfig.Upload.AllowedTypes,
		URLPrefix:     appConfig.Upload.URLPrefix,
		BaseURL:       appConfig.Server.BaseURL,
		DefaultAvatar: appConfig.Upload.DefaultAvatar,
	}
	utils.InitImageUploader(imageConfig)

	logger.Info("System", "应用初始化完成")
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.LogLevelDebug
	case "info":
		return logger.LogLevelInfo
	case "warn":
		return logger.LogLevelWarn
	case "error":
		return logger.LogLevelError
	default:
		return logger.LogLevelInfo
	}
}

func main() {
	configPath := flag.String("config", "./config.yaml", "后端 YAML 配置路径")
	flag.Parse()
	initialize(*configPath)
	defer dbManager.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	caps := socialservice.DefaultCapabilities()
	done := server.StartBackground(ctx, dbManager, caps)
	httpServer := &http.Server{Addr: appConfig.GetServerAddress(), Handler: server.NewRouter(dbManager, caps), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print("HTTP server failed")
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdown)
	_ = websocketmsg.Shutdown(shutdown)
	<-done
}
