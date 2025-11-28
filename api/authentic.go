package api

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
			Name:          loginReq.Username,
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

func HandleLogout(c *gin.Context) {

}

func HandleRegister(c *gin.Context) error {
	var registerReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Nickname string `json:"nickname" binding:"required"`
	}

	if err := c.ShouldBindJSON(&registerReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return err
	}

	user := models.User{
		UserId:        "U123456789",
		Username:      registerReq.Username,
		Nickname:      registerReq.Nickname,
		Name:          registerReq.Username,
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
	return nil

}

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
func HandleRefresh(c *gin.Context) {

}

func HandleChangePassword(c *gin.Context) {

}
