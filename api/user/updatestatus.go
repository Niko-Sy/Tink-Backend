package user

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UpdateStatusRequest struct {
	OnlineStatus string `json:"onlineStatus" binding:"required"`
}

func HandleUpdateUserStatus(c *gin.Context) {
	// 从JWT中间件获取用户ID
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录，请先登录获取Token",
		})
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证状态值
	var onlineStatus sqlcdb.UserOnlineStatus
	switch req.OnlineStatus {
	case "online":
		onlineStatus = sqlcdb.UserOnlineStatusOnline
	case "offline":
		onlineStatus = sqlcdb.UserOnlineStatusOffline
	case "away":
		onlineStatus = sqlcdb.UserOnlineStatusAway
	case "busy", "do_not_disturb":
		onlineStatus = sqlcdb.UserOnlineStatusDoNotDisturb
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的在线状态，支持: online, offline, away, busy",
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

	// 更新在线状态
	err = queries.UpdateUserOnlineStatus(c.Request.Context(), sqlcdb.UpdateUserOnlineStatusParams{
		UserID: currentUserID,
		OnlineStatus: sqlcdb.NullUserOnlineStatus{
			UserOnlineStatus: onlineStatus,
			Valid:            true,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新在线状态失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "状态更新成功",
		"data": gin.H{
			"onlineStatus": req.OnlineStatus,
		},
	})
}
