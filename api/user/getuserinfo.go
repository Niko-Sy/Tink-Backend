package user

import (
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UserInfoResponse struct {
	UserId            string     `json:"userId"`
	Username          string     `json:"username"`
	Nickname          string     `json:"nickname"`
	Avatar            string     `json:"avatar"`
	Email             string     `json:"email"`
	Phone             string     `json:"phone"`
	Signature         string     `json:"signature"`
	OnlineStatus      string     `json:"onlineStatus"`
	AccountStatus     string     `json:"accountStatus"`
	SystemRole        string     `json:"systemRole"`
	GlobalMuteStatus  string     `json:"globalMuteStatus"`
	GlobalMuteEndTime *time.Time `json:"globalMuteEndTime"`
	RegisterTime      time.Time  `json:"registerTime"`
	LastLoginTime     *time.Time `json:"lastLoginTime"`
}

func HandleGetUserInfo(c *gin.Context) {
	// 从JWT中间件获取用户ID
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录，请先登录获取Token",
		})
		return
	}

	// 获取数据库查询对象
	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据库连接失败",
			"error":   err.Error(),
		})
		return
	}

	// 查询用户信息
	user, err := queries.GetUserByID(c.Request.Context(), currentUserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "用户不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询用户信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	response := UserInfoResponse{
		UserId:            user.UserID,
		Username:          user.Username,
		Nickname:          user.Nickname.String,
		Avatar:            user.AvatarUrl.String,
		Email:             user.Email.String,
		Phone:             user.PhoneNumber.String,
		Signature:         user.Bio.String,
		OnlineStatus:      string(user.OnlineStatus.UserOnlineStatus),
		AccountStatus:     string(user.AccountStatus.UserAccountStatus),
		SystemRole:        string(user.SystemRole.UserSystemRole),
		GlobalMuteStatus:  "unmuted", // TODO: 从 user_settings 表获取
		GlobalMuteEndTime: nil,       // TODO: 从 user_settings 表获取
		RegisterTime:      user.RegisteredAt,
		LastLoginTime: func() *time.Time {
			if user.LastLoginAt.Valid {
				return &user.LastLoginAt.Time
			}
			return nil
		}(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": response,
	})
}

// 公开的用户信息响应（不含敏感信息）
type PublicUserInfoResponse struct {
	UserId        string `json:"userId"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	Signature     string `json:"signature"`
	OnlineStatus  string `json:"onlineStatus"`
	AccountStatus string `json:"accountStatus"`
	SystemRole    string `json:"systemRole"`
}

// HandleGetUserInfoByID 根据ID获取用户信息（公开接口）
func HandleGetUserInfoByID(c *gin.Context) {
	// 获取用户ID
	userID := c.Param("userid")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "用户ID不能为空",
		})
		return
	}

	// 获取数据库查询对象
	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据库连接失败",
			"error":   err.Error(),
		})
		return
	}

	// 查询用户信息
	user, err := queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "用户不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询用户信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应（只返回公开信息，不包含邮箱、电话等敏感信息）
	response := PublicUserInfoResponse{
		UserId:        user.UserID,
		Nickname:      user.Nickname.String,
		Avatar:        user.AvatarUrl.String,
		Signature:     user.Bio.String,
		OnlineStatus:  string(user.OnlineStatus.UserOnlineStatus),
		AccountStatus: string(user.AccountStatus.UserAccountStatus),
		SystemRole:    string(user.SystemRole.UserSystemRole),
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": response,
	})
}
