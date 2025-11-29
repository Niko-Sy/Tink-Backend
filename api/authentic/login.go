package authentic

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	UserId        string     `json:"userId"`
	Username      string     `json:"username"`
	Nickname      string     `json:"nickname"`
	Email         string     `json:"email"`
	Phone         string     `json:"phone"`
	Avatar        string     `json:"avatar"`
	Bio           string     `json:"bio"`
	OnlineStatus  string     `json:"onlineStatus"`
	AccountStatus string     `json:"accountStatus"`
	SystemRole    string     `json:"systemRole"`
	RegisterTime  time.Time  `json:"registerTime"`
	LastLoginTime *time.Time `json:"lastLoginTime"`
}

func HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
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

	// 根据用户名查找用户
	user, err := queries.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "用户名或密码错误",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询用户失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查账号状态
	if user.AccountStatus.UserAccountStatus == sqlcdb.UserAccountStatusSuspended {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "账号已被封禁",
		})
		return
	}
	if user.AccountStatus.UserAccountStatus == sqlcdb.UserAccountStatusDeleted {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
		})
		return
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
		})
		return
	}

	// 更新用户在线状态为online
	_ = queries.UpdateUserOnlineStatus(c.Request.Context(), sqlcdb.UpdateUserOnlineStatusParams{
		UserID: user.UserID,
		OnlineStatus: sqlcdb.NullUserOnlineStatus{
			UserOnlineStatus: sqlcdb.UserOnlineStatusOnline,
			Valid:            true,
		},
	})

	// 生成JWT token
	token, err := middleware.GenerateToken(user.UserID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成Token失败",
			"error":   err.Error(),
		})
		return
	}

	// 生成Refresh Token
	refreshToken, err := middleware.GenerateRefreshToken(user.UserID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成RefreshToken失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	response := LoginResponse{
		UserId:        user.UserID,
		Username:      user.Username,
		Nickname:      user.Nickname.String,
		Email:         user.Email.String,
		Phone:         user.PhoneNumber.String,
		Avatar:        user.AvatarUrl.String,
		Bio:           user.Bio.String,
		OnlineStatus:  "online",
		AccountStatus: string(user.AccountStatus.UserAccountStatus),
		SystemRole:    string(user.SystemRole.UserSystemRole),
		RegisterTime:  user.RegisteredAt,
		LastLoginTime: func() *time.Time {
			if user.LastLoginAt.Valid {
				return &user.LastLoginAt.Time
			}
			return nil
		}(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"user":         response,
			"token":        token,
			"refreshToken": refreshToken,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
