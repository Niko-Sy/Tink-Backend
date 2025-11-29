package chatroom

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UpdateChatRoomRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Type        *string `json:"type"`     // "public" | "private" | "protected"
	Password    *string `json:"password"` // 可选
}

type UpdateChatRoomResponse struct {
	RoomId          string    `json:"roomId"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Icon            string    `json:"icon"`
	Type            string    `json:"type"`
	OnlineCount     int32     `json:"onlineCount"`
	PeopleCount     int32     `json:"peopleCount"`
	CreatedTime     time.Time `json:"createdTime"`
	LastMessageTime time.Time `json:"lastMessageTime"`
}

func HandleUpdateRoom(c *gin.Context) {
	// 获取聊天室ID
	roomId := c.Param("roomid")
	if roomId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "聊天室ID不能为空",
		})
		return
	}

	var req UpdateChatRoomRequest
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

	// 检查用户权限（必须是房主或管理员）
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

	// 检查是否有管理权限
	if membership.MemberRole != sqlcdb.MemberRoleOwner && membership.MemberRole != sqlcdb.MemberRoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "没有权限修改聊天室信息，需要管理员或房主权限",
		})
		return
	}

	// 转换前端类型到数据库类型
	var roomType sqlcdb.ChatroomType
	if req.Type != nil {
		switch *req.Type {
		case "public":
			roomType = sqlcdb.ChatroomTypePublic
		case "private":
			roomType = sqlcdb.ChatroomTypePrivateInviteOnly
		case "protected":
			roomType = sqlcdb.ChatroomTypePrivatePassword
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无效的聊天室类型，支持: public, private, protected",
			})
			return
		}
	}

	// 构建更新参数
	updateParams := sqlcdb.UpdateChatroomParams{
		RoomID: roomId,
	}

	if req.Name != nil {
		updateParams.RoomName = *req.Name
	}
	if req.Description != nil {
		updateParams.Description = sql.NullString{String: *req.Description, Valid: true}
	}
	if req.Icon != nil {
		updateParams.IconUrl = sql.NullString{String: *req.Icon, Valid: true}
	}
	if req.Type != nil {
		updateParams.RoomType = roomType
	}
	if req.Password != nil {
		updateParams.AccessPassword = sql.NullString{String: *req.Password, Valid: true}
	}

	// 执行更新
	updatedRoom, err := queries.UpdateChatroom(c.Request.Context(), updateParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 转换聊天室类型为前端格式
	var responseType string
	switch updatedRoom.RoomType {
	case sqlcdb.ChatroomTypePublic:
		responseType = "public"
	case sqlcdb.ChatroomTypePrivateInviteOnly:
		responseType = "private"
	case sqlcdb.ChatroomTypePrivatePassword:
		responseType = "protected"
	default:
		responseType = string(updatedRoom.RoomType)
	}

	// 构建响应
	response := UpdateChatRoomResponse{
		RoomId:      updatedRoom.RoomID,
		Name:        updatedRoom.RoomName,
		Description: updatedRoom.Description.String,
		Icon:        updatedRoom.IconUrl.String,
		Type:        responseType,
		OnlineCount: updatedRoom.OnlineCount,
		PeopleCount: updatedRoom.MemberCount,
		CreatedTime: updatedRoom.CreatedAt,
		LastMessageTime: func() time.Time {
			if updatedRoom.LastActiveAt.Valid {
				return updatedRoom.LastActiveAt.Time
			}
			return updatedRoom.CreatedAt
		}(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "更新成功",
		"data":    response,
	})
}
