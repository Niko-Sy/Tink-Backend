package main

import (
	"chatroombackend/api"

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
			authGroup.POST("/login", api.HandleLogin)
			authGroup.POST("/register", api.HandleRegister)
			authGroup.POST("/logout", api.HandleLogout)
			authGroup.POST("/refresh", api.HandleRefresh)
			authGroup.POST("/changepwd", api.HandleChangePassword)
		}
	}

	err := router.Run(":8080")
	if err != nil {
		return
	}
}
