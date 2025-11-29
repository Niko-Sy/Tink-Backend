package main

import (
	"chatroombackend/api/authentic"
	"chatroombackend/api/chatroom"
	"chatroombackend/api/user"
	"chatroombackend/middleware"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL 驱动
)

var dbManager *middleware.DBManager

func init() {
	// 初始化数据库配置
	config := middleware.DefaultDBConfig()
	config.DSN = getEnvOrDefault("DATABASE_URL", "postgres://postgres:123456@localhost:5432/chatroom?sslmode=disable")

	var err error
	dbManager, err = middleware.NewDBManager(config)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
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

	// 使用数据库中间件
	router.Use(middleware.DBMiddleware(dbManager))

	// 数据库健康检查端点
	router.GET("/health/db", middleware.DBStatusHandler(dbManager))

	apiV1 := router.Group("/api/v1")
	{
		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/login", authentic.HandleLogin)
			authGroup.POST("/register", authentic.HandleRegister)
			authGroup.GET("/logout", authentic.HandleLogout)
			authGroup.GET("/refresh", authentic.HandleRefresh)
			authGroup.POST("/changepwd", authentic.HandleChangePassword)
			// authGroup.GET("/userinfo", authentic.HandleGetUserInfo)
			// authGroup.POST("/updateuserinfo", authentic.HandleUpdateUserInfo)
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
			}
		}
		usersGroup := apiV1.Group("/users")
		{
			userAuth := usersGroup.Group("")
			userAuth.Use(middleware.JWTAuthMiddleware())
			{
				// userAuth.GET("/me/userinfo", user.HandleGetUserInfo)
				// userAuth.POST("/me/updateuserinfo", user.HandleUpdateUserInfo)
				// userAuth.POST("/me/updatestatus", user.HandleUpdateUserStatus)
				userAuth.GET("/me/chatrooms", user.HandleGetUserChatrooms)
			}
		}

		// 需要事务支持的路由组示例
		// transactionGroup := apiV1.Group("/transaction")
		// transactionGroup.Use(middleware.TransactionMiddleware(dbManager))
		// {
		//     // 在此组中的所有请求都会自动开启事务
		// }
	}

	err := router.Run(":8080")
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
