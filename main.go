package main

import (
	"chatroombackend/api/authentic"
	"chatroombackend/api/chatroom"
	"chatroombackend/api/member"
	"chatroombackend/api/messages"
	"chatroombackend/api/user"
	"chatroombackend/api/websocketmsg"
	"chatroombackend/config"
	"chatroombackend/logger"
	"chatroombackend/middleware"
	"chatroombackend/utils"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL 驱动
)

var dbManager *middleware.DBManager
var appConfig *config.Config

func init() {
	// 加载配置文件
	var err error
	configPath := getEnvOrDefault("CONFIG_PATH", "./config.yaml")
	appConfig, err = config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

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
	// 优雅关闭
	defer func() {
		if dbManager != nil {
			dbManager.Close()
		}
	}()

	// 监听系统信号，实现优雅关闭
	go handleShutdown()

	router := gin.Default()

	// CORS 中间件
	router.Use(middleware.CORSMiddleware())

	// 使用数据库中间件
	router.Use(middleware.DBMiddleware(dbManager))

	// 注入 sql queries 到 websocket 包以支持消息入库与房间管理
	websocketmsg.SetQueries(dbManager.GetQueries())

	// 图片静态文件服务
	utils.ServeStaticImages(router, "/static/images", "./uploads")

	// 数据库健康检查端点
	router.GET("/health/db", middleware.DBStatusHandler(dbManager))

	// WebSocket 实时通信接口
	router.GET("/ws", websocketmsg.HandleWebSocket)

	apiV1 := router.Group("/api/v1")
	{

		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/login", authentic.HandleLogin)
			authGroup.POST("/register", authentic.HandleRegister)
			authTokenGroup := authGroup.Group("")
			authTokenGroup.Use(middleware.JWTAuthMiddleware())
			{
				authTokenGroup.GET("/logout", authentic.HandleLogout)
				authTokenGroup.GET("/refresh", authentic.HandleRefresh)
				authTokenGroup.POST("/changepwd", authentic.HandleChangePassword)
			}

		}
		chatroomGroup := apiV1.Group("/chatroom")
		{
			// 公开接口（不需要登录）
			chatroomGroup.GET("/:roomid/info", chatroom.HandleGetRoomInfo)

			// 需要登录的接口
			chatroomAuth := chatroomGroup.Group("")
			chatroomAuth.Use(middleware.JWTAuthMiddleware())
			{
				chatroomAuth.POST("/createroom", chatroom.HandleCreateRoom)
				chatroomAuth.POST("/joinroom", chatroom.HandleJoinRoom)
				chatroomAuth.POST("/leaveroom", chatroom.HandleLeaveRoom)
				chatroomAuth.POST("/:roomid/update", chatroom.HandleUpdateRoom)
				chatroomAuth.POST("/:roomid/delete", chatroom.HandleDeleteRoom)
				// 聊天室图片上传
				chatroomAuth.POST("/:roomid/uploadimage", utils.HandleUploadChatImage)

				// 消息相关接口
				chatroomAuth.POST("/:roomid/messages", messages.HandleSendMessage)
				chatroomAuth.GET("/:roomid/messages", messages.HandleGetMessageHistory)
				chatroomAuth.POST("/:roomid/messages/:messageid/edit", messages.HandleEditMessage)
				chatroomAuth.POST("/:roomid/messages/:messageid/delete", messages.HandleDeleteMessage)

				membersgroup := chatroomAuth.Group("/:roomid/members")
				{
					membersgroup.GET("/memberlist", member.HandleListRoomMembers)
					membersgroup.GET("/search", member.HandleSearchRoomMembers)
					membersgroup.GET("/:userid/info", member.HandleGetRoomMemberInfo)
					membersgroup.POST("/kick", member.HandleKickRoomMember)
					membersgroup.POST("/mute", member.HandleMuteRoomMember)
					membersgroup.POST("/unmute", member.HandleUnmuteRoomMember)
					membersgroup.POST("/setadmin", member.HandleSetAdminRoomMember)
					membersgroup.POST("/removeadmin", member.HandleRemoveAdminRoomMember)
				}
			}
		}
		usersGroup := apiV1.Group("/users")
		{
			usersGroup.GET("/:userid/info", user.HandleGetUserInfoByID)

			userAuth := usersGroup.Group("")
			userAuth.Use(middleware.JWTAuthMiddleware())
			{
				userAuth.GET("/me/userinfo", user.HandleGetUserInfo)
				userAuth.POST("/me/update", user.HandleUpdateUserInfo)
				userAuth.POST("/me/updatestatus", user.HandleUpdateUserStatus)
				userAuth.GET("/me/chatrooms", user.HandleGetUserChatrooms)
				// 用户头像上传
				userAuth.POST("/me/uploadavatar", utils.HandleUploadAvatar)
			}
		}

		// 需要事务支持的路由组示例
		// transactionGroup := apiV1.Group("/transaction")
		// transactionGroup.Use(middleware.TransactionMiddleware(dbManager))
		// {
		//     // 在此组中的所有请求都会自动开启事务
		// }
	}

	err := router.Run(appConfig.GetServerAddress())
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

// handleShutdown 处理系统关闭信号
func handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭服务器...")
	if dbManager != nil {
		dbManager.Close()
	}
	os.Exit(0)
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
