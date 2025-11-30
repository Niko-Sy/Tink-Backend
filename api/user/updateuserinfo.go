package user

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UpdateUserInfoRequest struct {
	Nickname  *string `json:"nickname"`
	Avatar    *string `json:"avatar"`
	Signature *string `json:"signature"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
}

type UpdateUserInfoResponse struct {
	UserId        string     `json:"userId"`
	Username      string     `json:"username"`
	Nickname      string     `json:"nickname"`
	Avatar        string     `json:"avatar"`
	Email         string     `json:"email"`
	Phone         string     `json:"phone"`
	Signature     string     `json:"signature"`
	OnlineStatus  string     `json:"onlineStatus"`
	AccountStatus string     `json:"accountStatus"`
	SystemRole    string     `json:"systemRole"`
	RegisterTime  time.Time  `json:"registerTime"`
	LastLoginTime *time.Time `json:"lastLoginTime"`
}

func HandleUpdateUserInfo(c *gin.Context) {
	// 从 JWT 中间件获取用户ID
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录，请先登录获取Token",
		})
		return
	}

	var req UpdateUserInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查是否有要更新的字段
	if req.Nickname == nil && req.Avatar == nil && req.Signature == nil && req.Phone == nil && req.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "没有要更新的字段",
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

	// 检查用户是否存在
	_, err = queries.GetUserByID(c.Request.Context(), currentUserID)
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

	// 构建更新参数
	updateParams := sqlcdb.UpdateUserParams{
		UserID: currentUserID,
	}

	if req.Nickname != nil {
		updateParams.Nickname = sql.NullString{String: *req.Nickname, Valid: true}
	}
	if req.Avatar != nil {
		updateParams.AvatarUrl = sql.NullString{String: *req.Avatar, Valid: true}
	}
	if req.Signature != nil {
		updateParams.Bio = sql.NullString{String: *req.Signature, Valid: true}
	}
	if req.Phone != nil {
		updateParams.PhoneNumber = sql.NullString{String: *req.Phone, Valid: true}
	}
	if req.Email != nil {
		updateParams.Email = sql.NullString{String: *req.Email, Valid: true}
	}

	// 执行更新
	updatedUser, err := queries.UpdateUser(c.Request.Context(), updateParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新用户信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	response := UpdateUserInfoResponse{
		UserId:        updatedUser.UserID,
		Username:      updatedUser.Username,
		Nickname:      updatedUser.Nickname.String,
		Avatar:        updatedUser.AvatarUrl.String,
		Email:         updatedUser.Email.String,
		Phone:         updatedUser.PhoneNumber.String,
		Signature:     updatedUser.Bio.String,
		OnlineStatus:  string(updatedUser.OnlineStatus.UserOnlineStatus),
		AccountStatus: string(updatedUser.AccountStatus.UserAccountStatus),
		SystemRole:    string(updatedUser.SystemRole.UserSystemRole),
		RegisterTime:  updatedUser.RegisteredAt,
		LastLoginTime: func() *time.Time {
			if updatedUser.LastLoginAt.Valid {
				return &updatedUser.LastLoginAt.Time
			}
			return nil
		}(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    response,
	})
}
