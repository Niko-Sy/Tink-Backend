package chatroom

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type LeaveChatRoomRequest struct {
	RoomId string `json:"roomId" binding:"required"`
}

func HandleLeaveRoom(c *gin.Context) {
	var req LeaveChatRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
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
	chatroom, err := queries.GetChatroomByID(c.Request.Context(), req.RoomId)
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

	// 检查用户是否在聊天室中
	isInRoom, err := queries.IsUserInChatroom(c.Request.Context(), sqlcdb.IsUserInChatroomParams{
		UserID: currentUserID,
		RoomID: req.RoomId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "检查成员状态失败",
			"error":   err.Error(),
		})
		return
	}
	if !isInRoom {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "您不是该聊天室成员",
		})
		return
	}

	// 获取用户在聊天室中的成员信息，检查是否是房主
	membership, err := queries.GetUserChatroomMembership(c.Request.Context(), sqlcdb.GetUserChatroomMembershipParams{
		UserID: currentUserID,
		RoomID: req.RoomId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取成员信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 房主不能直接退出聊天室，需要先转让房主或解散聊天室
	if membership.MemberRole == sqlcdb.MemberRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "房主不能直接退出聊天室，请先转让房主权限或解散聊天室",
		})
		return
	}

	// 执行退出操作
	err = queries.LeaveChatroom(c.Request.Context(), sqlcdb.LeaveChatroomParams{
		UserID: currentUserID,
		RoomID: req.RoomId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "退出聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 减少聊天室成员计数
	err = queries.DecrementChatroomMemberCount(c.Request.Context(), req.RoomId)
	if err != nil {
		// 记录日志但不影响响应
		c.Error(err)
	}

	_ = chatroom // 避免未使用变量警告，后续可用于日志等

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "退出成功",
	})
}
