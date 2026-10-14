package server

import (
	"chatroombackend/api/authentic"
	"chatroombackend/api/chatroom"
	groupapi "chatroombackend/api/group"
	invitationapi "chatroombackend/api/invitation"
	"chatroombackend/api/member"
	"chatroombackend/api/messages"
	"chatroombackend/api/user"
	"chatroombackend/api/websocketmsg"
	"chatroombackend/internal/telemetry"
	"chatroombackend/middleware"
	groupsvc "chatroombackend/service/group"
	invitesvc "chatroombackend/service/invitation"
	"chatroombackend/utils"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"log"
	"time"

	directapi "chatroombackend/api/direct"
	socialapi "chatroombackend/api/social"
	directservice "chatroombackend/service/direct"
	messagecore "chatroombackend/service/message"
	socialservice "chatroombackend/service/social"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // PostgreSQL 驱动
)

// NewRouter builds routes without opening a database or starting a listener.
func NewRouter(dbManager *middleware.DBManager, capabilities socialservice.Capabilities) *gin.Engine {
	directService := directservice.New(dbManager.GetDB(), dbManager.GetQueries(), capabilities.DirectMessages, websocketmsg.PublishSocialEvent)
	messageService := messagecore.New(dbManager.GetDB(), dbManager.GetQueries(), directService, websocketmsg.PublishRoomEvent)
	socialService := socialservice.New(dbManager.GetDB(), dbManager.GetQueries(), capabilities, websocketmsg.PublishSocialEvent)
	groupService := groupsvc.New(dbManager.GetDB(), dbManager.GetQueries(), websocketmsg.PublishGroupEvent)
	invitationService := invitesvc.New(dbManager.GetDB(), dbManager.GetQueries(), capabilities.RoomInvitations, groupService, websocketmsg.PublishSocialEvent)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("invitationService", invitationService); c.Next() })
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		started := time.Now()
		trace := uuid.NewString()
		c.Header("X-Request-ID", trace)
		c.Next()
		if c.Request.Context().Err() != nil {
			return
		}
		if (c.Request.Method == "GET" && c.Writer.Status() < 400 && time.Since(started) < 500*time.Millisecond) || c.FullPath() == "" || c.FullPath() == "/ws" || c.FullPath() == "/health/db" {
			return
		}
		telemetry.Record(fmt.Sprintf("%s %s/%d/%s", c.Request.Method, c.FullPath(), c.Writer.Status(), c.GetString("businessCode")))
		encoded, _ := json.Marshal(map[string]any{"traceId": trace, "operation": c.Request.Method + " " + c.FullPath(), "status": c.Writer.Status(), "businessCode": c.GetString("businessCode"), "durationMs": time.Since(started).Milliseconds(), "resourceId": c.Param("roomid") + c.Param("invitationid")})
		log.Print(string(encoded))
	})

	// CORS 中间件
	router.Use(middleware.CORSMiddleware())

	// 使用数据库中间件
	router.Use(middleware.DBMiddleware(dbManager))
	router.Use(func(c *gin.Context) { c.Set("messageService", messageService); c.Next() })

	// 注入 sql queries 到 websocket 包以支持消息入库与房间管理
	websocketmsg.SetRuntime(dbManager.GetQueries(), dbManager.GetDB())

	// 图片静态文件服务
	imageConfig := utils.GetImageUploader().GetConfig()
	utils.ServeStaticImages(router, imageConfig.URLPrefix, imageConfig.UploadDir)

	// 数据库健康检查端点
	router.GET("/health/db", middleware.DBStatusHandler(dbManager))

	router.GET("/health/ready", func(c *gin.Context) {
		if err := dbManager.GetDB().PingContext(c.Request.Context()); err != nil {
			c.JSON(503, gin.H{"ready": false})
			return
		}
		c.JSON(200, gin.H{"ready": true})
	})
	// WebSocket 实时通信接口
	tickets := websocketmsg.NewTicketStore()
	router.GET("/ws", websocketmsg.HandleWithMessages(messageService, tickets))
	router.POST("/api/v1/auth/ws-ticket", middleware.JWTAuthMiddleware(), tickets.Mint)

	apiV1 := router.Group("/api/v1")
	{

		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/login", authentic.HandleLogin)
			authGroup.POST("/register", authentic.HandleRegister)
			authGroup.POST("/refresh", authentic.HandleRefresh)
			authGroup.GET("/refresh", authentic.HandleRefresh)
			authTokenGroup := authGroup.Group("")
			authTokenGroup.Use(middleware.JWTAuthMiddleware())
			{
				authTokenGroup.GET("/logout", authentic.HandleLogout)

				authTokenGroup.POST("/changepwd", authentic.HandleChangePassword)
			}

		}
		chatroomGroup := apiV1.Group("/chatroom")
		{
			// 公开接口（不需要登录）
			chatroomGroup.GET("/:roomid/info", middleware.GroupConversationGuard(), chatroom.HandleGetRoomInfo)

			// 需要登录的接口
			chatroomAuth := chatroomGroup.Group("")
			chatroomAuth.Use(middleware.JWTAuthMiddleware(), middleware.GroupConversationGuard())
			{
				groupapi.Register(chatroomAuth, groupService)

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

					membersgroup.POST("/mute", member.HandleMuteRoomMember)
					membersgroup.POST("/unmute", member.HandleUnmuteRoomMember)

				}
			}
		}
		usersGroup := apiV1.Group("/users")
		{
			usersGroup.GET("/:userid/info", socialapi.PublicProfile(socialService))

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

		socialapi.Register(apiV1, socialService)
		directapi.Register(apiV1, directService, socialService)
		invitationapi.Register(apiV1, invitationService)

		// 需要事务支持的路由组示例
		// transactionGroup := apiV1.Group("/transaction")
		// transactionGroup.Use(middleware.TransactionMiddleware(dbManager))
		// {
		//     // 在此组中的所有请求都会自动开启事务
		// }
	}

	return router
}
