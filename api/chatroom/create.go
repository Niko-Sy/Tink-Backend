package chatroom

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type CreateChatRoomRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Type        string `json:"type" binding:"required,oneof=public private protected"`
	Password    string `json:"password"`
}

type CreateChatRoomResponse struct {
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

type MemberInfoResponse struct {
	MemberId string    `json:"memberId"`
	RoomId   string    `json:"roomId"`
	UserId   string    `json:"userId"`
	RoomRole string    `json:"roomRole"`
	IsMuted  bool      `json:"isMuted"`
	JoinedAt time.Time `json:"joinedAt"`
	IsActive bool      `json:"isActive"`
}

func HandleCreateRoom(c *gin.Context) {
	var req CreateChatRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证 protected 类型必须有密码
	if req.Type == "protected" && req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "protected类型的聊天室必须设置密码",
		})
		return
	}

	// 从JWT中间件设置的上下文获取用户ID
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

	// 转换聊天室类型
	var roomType sqlcdb.ChatroomType
	switch req.Type {
	case "public":
		roomType = sqlcdb.ChatroomTypePublic
	case "private":
		roomType = sqlcdb.ChatroomTypePrivateInviteOnly
	case "protected":
		roomType = sqlcdb.ChatroomTypePrivatePassword
	default:
		roomType = sqlcdb.ChatroomTypePublic
	}

	// 创建聊天室参数
	createParams := sqlcdb.CreateChatroomParams{
		RoomName: req.Name,
		Description: sql.NullString{
			String: req.Description,
			Valid:  req.Description != "",
		},
		IconUrl: sql.NullString{
			String: req.Icon,
			Valid:  req.Icon != "",
		},
		RoomType: roomType,
		AccessPassword: sql.NullString{
			String: req.Password,
			Valid:  req.Password != "",
		},
	}

	// 创建聊天室
	chatroom, err := queries.CreateChatroom(c.Request.Context(), createParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 创建房主成员记录
	joinParams := sqlcdb.JoinChatroomParams{
		UserID:     currentUserID,
		RoomID:     chatroom.RoomID,
		MemberRole: sqlcdb.MemberRoleOwner,
	}

	member, err := queries.JoinChatroom(c.Request.Context(), joinParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建房主记录失败",
			"error":   err.Error(),
		})
		return
	}

	// 增加聊天室成员计数
	err = queries.IncrementChatroomMemberCount(c.Request.Context(), chatroom.RoomID)
	if err != nil {
		// 记录日志但不影响响应
		c.Error(err)
	}

	// 构建响应
	response := CreateChatRoomResponse{
		RoomId:      chatroom.RoomID,
		Name:        chatroom.RoomName,
		Description: chatroom.Description.String,
		Icon:        chatroom.IconUrl.String,
		Type:        req.Type,
		CreatorId:   currentUserID,
		OnlineCount: chatroom.OnlineCount,
		PeopleCount: chatroom.MemberCount + 1, // 加上刚创建的房主
		CreatedTime: chatroom.CreatedAt,
		LastMessageTime: func() time.Time {
			if chatroom.LastActiveAt.Valid {
				return chatroom.LastActiveAt.Time
			}
			return chatroom.CreatedAt
		}(),
	}

	memberInfo := MemberInfoResponse{
		MemberId: member.MemberRelID,
		RoomId:   member.RoomID,
		UserId:   member.UserID,
		RoomRole: string(member.MemberRole),
		IsMuted:  member.MuteStatus == sqlcdb.MemberMuteStatusMuted,
		JoinedAt: member.JoinedAt,
		IsActive: member.IsActive,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建成功",
		"data": gin.H{
			"chatroom":   response,
			"memberInfo": memberInfo,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
