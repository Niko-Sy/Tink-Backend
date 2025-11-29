package main

import (
	"chatroombackend/api/authentic"
	"chatroombackend/api/chatroom"

	"github.com/gin-gonic/gin"
)

func init() {

}
func main() {
	router := gin.Default()
	apiV1 := router.Group("/api/v1")
	{
		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/login", authentic.HandleLogin)
			authGroup.POST("/register", authentic.HandleRegister)
			authGroup.GET("/logout", authentic.HandleLogout)
			authGroup.GET("/refresh", authentic.HandleRefresh)
			authGroup.POST("/changepwd", authentic.HandleChangePassword)
			authGroup.GET("/userinfo", authentic.HandleGetUserInfo)
			authGroup.POST("/updateuserinfo", authentic.HandleUpdateUserInfo)
		}
		chatroomGroup := apiV1.Group("/chatroom")
		{
			chatroomGroup.GET("/getroomlist", chatroom.HandleGetRoomList)
			chatroomGroup.POST("/createroom", chatroom.HandleCreateRoom)
			chatroomGroup.POST("/joinroom", chatroom.HandleJoinRoom)
			chatroomGroup.POST("/leaveroom", chatroom.HandleLeaveRoom)
			chatroomGroup.GET("/:roomid/info", chatroom.HandleGetRoomInfo)
		}
	}

	err := router.Run(":8080")
	if err != nil {
		return
	}
}
