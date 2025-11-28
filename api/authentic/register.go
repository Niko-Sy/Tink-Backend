package authentic

import (
	"chatroombackend/models"
	"chatroombackend/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func HandleRegister(c *gin.Context) {
	var registerReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Nickname string `json:"nickname" binding:"required"`
	}

	if err := c.ShouldBindJSON(&registerReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	user := models.User{
		UserId:        utils.GenerateUserID(),
		Username:      registerReq.Username,
		Nickname:      registerReq.Nickname,
		Avatar:        "https://example.com/avatar.jpg",
		Email:         registerReq.Email,
		Password:      registerReq.Password,
		OnlineStatus:  "online",
		AccountStatus: "active",
		SystemRole:    "user",
		RegisterTime:  time.Now().AddDate(0, 0, -30), // 30天前注册
		LastLoginTime: time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "注册成功",
		"user":    user,
		"token":   "fake-jwt-token-for-" + registerReq.Username,
	})
	return

}
