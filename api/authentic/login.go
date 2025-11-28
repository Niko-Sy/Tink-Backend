package authentic

import (
	"chatroombackend/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func HandleLogin(c *gin.Context) {
	// 模拟登录处理
	var loginReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 模拟验证用户凭据
	// 实际应用中这里应该查询数据库验证用户名和密码
	if loginReq.Username == "admin" && loginReq.Password == "password" {
		// 创建模拟用户
		user := models.User{
			UserId:        "U123456789",
			Username:      loginReq.Username,
			Nickname:      "管理员",
			Avatar:        "https://example.com/avatar.jpg",
			Email:         "admin@example.com",
			OnlineStatus:  "online",
			AccountStatus: "active",
			SystemRole:    "super_admin",
			RegisterTime:  time.Now().AddDate(0, 0, -30), // 30天前注册
			LastLoginTime: time.Now(),
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "登录成功",
			"user":    user,
			"token":   "fake-jwt-token-for-" + loginReq.Username,
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
}
