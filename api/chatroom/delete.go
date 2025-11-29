package chatroom

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleDeleteRoom(c *gin.Context) {
	// 获取聊天室ID
	roomId := c.Param("roomid")
	if roomId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "聊天室ID不能为空",
		})
		return
	}

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

	// 检查聊天室是否存在
	_, err = queries.GetChatroomByID(c.Request.Context(), roomId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "聊天室不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 检查用户权限（必须是房主）
	membership, err := queries.GetUserChatroomMembership(c.Request.Context(), sqlcdb.GetUserChatroomMembershipParams{
		UserID: currentUserID,
		RoomID: roomId,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "您不是该聊天室成员",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取成员信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 只有房主可以删除聊天室
	if membership.MemberRole != sqlcdb.MemberRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "没有权限删除聊天室，仅房主可以删除",
		})
		return
	}

	// 执行软删除
	err = queries.DeleteChatroom(c.Request.Context(), roomId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除成功",
	})
}
