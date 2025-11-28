package authentic

import (
	"chatroombackend/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func HandleUpdateUserInfo(context *gin.Context) {
	var updateUserInfoReq struct {
		UserId    string `json:"userId" binding:"required"` // 用户ID（U+9位数字）
		Username  string `json:"username"`                  // 用户名
		Nickname  string `json:"nickname,omitempty"`        // 昵称
		Avatar    string `json:"avatar"`                    // 头像URL
		Email     string `json:"email,omitempty"`           // 邮箱
		Phone     string `json:"phone,omitempty"`           // 手机号
		Signature string `json:"signature,omitempty"`       // 个性签名
	}
	if err := context.ShouldBindJSON(&updateUserInfoReq); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 模拟数据库获取原始用户信息
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

	// 检查是否存在改动
	hasChanges := false
	originalUser := user // 保存原始值用于比较

	if updateUserInfoReq.Username != "" && updateUserInfoReq.Username != originalUser.Username {
		user.Username = updateUserInfoReq.Username
		hasChanges = true
	}
	if updateUserInfoReq.Nickname != "" && updateUserInfoReq.Nickname != originalUser.Nickname {
		user.Nickname = updateUserInfoReq.Nickname
		hasChanges = true
	}
	if updateUserInfoReq.Avatar != "" && updateUserInfoReq.Avatar != originalUser.Avatar {
		user.Avatar = updateUserInfoReq.Avatar
		hasChanges = true
	}
	if updateUserInfoReq.Email != "" && updateUserInfoReq.Email != originalUser.Email {
		user.Email = updateUserInfoReq.Email
		hasChanges = true
	}
	if updateUserInfoReq.Phone != "" && updateUserInfoReq.Phone != originalUser.Phone {
		user.Phone = updateUserInfoReq.Phone
		hasChanges = true
	}
	if updateUserInfoReq.Signature != "" && updateUserInfoReq.Signature != originalUser.Signature {
		user.Signature = updateUserInfoReq.Signature
		hasChanges = true
	}

	// 如果没有改动，则返回相应提示
	if !hasChanges {
		context.JSON(http.StatusOK, gin.H{
			"message": "无改动需要更新",
			"user":    user,
		})
		return
	}

	// 如果有改动，执行更新逻辑（此处为模拟）
	context.JSON(http.StatusOK, gin.H{
		"message": "用户信息更新成功",
		"user":    user,
	})

}
