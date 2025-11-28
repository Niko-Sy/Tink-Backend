package authentic

import (
	"chatroombackend/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleGetUserInfo(c *gin.Context) {
	var getUserInfoReq struct {
		UserId string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&getUserInfoReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 模拟数据库查询

	UserInfo := models.UserInfo{
		UserId:           "U123456789",
		Username:         "admin",
		Nickname:         "管理员",
		Avatar:           "https://example.com/avatar.jpg",
		Email:            "admin@example.com",
		Phone:            "111111111111",
		Signature:        "I am a super admin",
		OnlineStatus:     "online",
		GlobalMuteStatus: "unmuted",
	}

	c.JSON(http.StatusOK, gin.H{"message": "获取用户信息成功", "data": UserInfo})
}
