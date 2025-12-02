package authentic

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

func HandleLogout(c *gin.Context) {
	queries, err := middleware.GetQueriesFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取数据库连接失败",
			"error":   err.Error(),
		})
		return
	}

	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户未登录",
		})
		return
	}

	err = queries.UpdateUserOnlineStatus(c.Request.Context(), sqlcdb.UpdateUserOnlineStatusParams{
		UserID: userID,
		OnlineStatus: sqlcdb.NullUserOnlineStatus{
			UserOnlineStatus: sqlcdb.UserOnlineStatusOffline,
			Valid:            true,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新用户在线状态失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登出成功",
	})
}
