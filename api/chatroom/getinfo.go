package chatroom

import (
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatRoomInfoResponse struct {
	RoomId          string    `json:"roomId"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Icon            string    `json:"icon"`
	Type            string    `json:"type"`
	CreatorId       string    `json:"creatorId"`
	OnlineCount     int32     `json:"onlineCount"`
	PeopleCount     int32     `json:"peopleCount"`
	CreatedTime     time.Time `json:"createdTime"`
	LastMessageTime time.Time `json:"lastMessageTime"`
}

func HandleGetRoomInfo(c *gin.Context) {
	// 获取聊天室ID
	roomId := c.Param("roomid")
	if roomId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "聊天室ID不能为空",
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

	// 查询聊天室信息（不含密码）
	chatroom, err := queries.GetChatroomWithoutPassword(c.Request.Context(), roomId)
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

	// 获取房主信息
	owner, err := queries.GetChatroomOwner(c.Request.Context(), roomId)
	creatorId := ""
	if err == nil {
		creatorId = owner.UserID
	}

	// 转换聊天室类型为前端格式
	var roomType string
	switch chatroom.RoomType {
	case "public":
		roomType = "public"
	case "private_invite_only":
		roomType = "private"
	case "private_password":
		roomType = "protected"
	default:
		roomType = string(chatroom.RoomType)
	}

	// 构建响应
	response := ChatRoomInfoResponse{
		RoomId:      chatroom.RoomID,
		Name:        chatroom.RoomName,
		Description: chatroom.Description.String,
		Icon:        chatroom.IconUrl.String,
		Type:        roomType,
		CreatorId:   creatorId,
		OnlineCount: chatroom.OnlineCount,
		PeopleCount: chatroom.MemberCount,
		CreatedTime: chatroom.CreatedAt,
		LastMessageTime: func() time.Time {
			if chatroom.LastActiveAt.Valid {
				return chatroom.LastActiveAt.Time
			}
			return chatroom.CreatedAt
		}(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      200,
		"message":   "success",
		"data":      response,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
