package main

import (
	"chatroombackend/api/authentic"

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
	}

	err := router.Run(":8080")
	if err != nil {
		return
	}
}
