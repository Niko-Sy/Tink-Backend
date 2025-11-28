package authentic

import (
	"chatroombackend/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func HandleChangePassword(c *gin.Context) {
	var changePasswordReq struct {
		UserId    string `json:"userId" binding:"required"`
		OldPasswd string `json:"oldPasswd" binding:"required"`
		NewPasswd string `json:"newPasswd" binding:"required"`
	}
	if err := c.ShouldBindJSON(&changePasswordReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	//模拟数据库查询和修改密码
	user := models.User{
		UserId:        "U123456789",
		Username:      "admin",
		Nickname:      "管理员",
		Avatar:        "https://example.com/avatar.jpg",
		Email:         "admin@example.com",
		Password:      "old-password",
		OnlineStatus:  "online",
		AccountStatus: "active",
		SystemRole:    "super_admin",
		RegisterTime:  time.Now().AddDate(0, 0, -30), // 30天前注册
		LastLoginTime: time.Now(),
	}
	if user.Password == changePasswordReq.OldPasswd {
		user.Password = changePasswordReq.NewPasswd
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "旧密码错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "修改密码成功"})

}
