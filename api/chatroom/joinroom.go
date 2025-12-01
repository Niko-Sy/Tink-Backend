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

type JoinChatRoomRequest struct {
	RoomId   string `json:"roomId" binding:"required"`
	Password string `json:"password"` // 仅protected类型需要
}

type JoinChatRoomResponse struct {
	Chatroom   ChatroomInfoResponse `json:"chatroom"`
	MemberInfo MemberInfoResponse   `json:"memberInfo"`
}

type ChatroomInfoResponse struct {
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

func HandleJoinRoom(c *gin.Context) {
	var req JoinChatRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	roomId := req.RoomId

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

	// 查询聊天室信息
	chatroom, err := queries.GetChatroomByID(c.Request.Context(), roomId)
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

	// 检查用户是否已经在聊天室中
	isInRoom, err := queries.IsUserInChatroom(c.Request.Context(), sqlcdb.IsUserInChatroomParams{
		UserID: currentUserID,
		RoomID: roomId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "检查成员状态失败",
			"error":   err.Error(),
		})
		return
	}
	if isInRoom {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "您已经是该聊天室成员",
		})
		return
	}

	// 根据聊天室类型验证
	switch chatroom.RoomType {
	case sqlcdb.ChatroomTypePrivateInviteOnly:
		// 私有聊天室（仅邀请）不允许直接加入
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "该聊天室为私有聊天室，需要邀请才能加入",
		})
		return

	case sqlcdb.ChatroomTypePrivatePassword:
		// 需要密码验证
		if req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "该聊天室需要密码才能加入",
			})
			return
		}

		// 验证密码
		isValid, err := queries.VerifyChatroomPassword(c.Request.Context(), sqlcdb.VerifyChatroomPasswordParams{
			RoomID: roomId,
			AccessPassword: sql.NullString{
				String: req.Password,
				Valid:  true,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "验证密码失败",
				"error":   err.Error(),
			})
			return
		}
		if !isValid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "密码错误",
			})
			return
		}

	case sqlcdb.ChatroomTypePublic:
		// 公开聊天室，无需验证
	}

	// 加入聊天室（默认角色为member）
	joinParams := sqlcdb.JoinChatroomParams{
		UserID:     currentUserID,
		RoomID:     roomId,
		MemberRole: sqlcdb.MemberRoleMember,
	}

	member, err := queries.JoinChatroom(c.Request.Context(), joinParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "加入聊天室失败",
			"error":   err.Error(),
		})
		return
	}

	// 增加聊天室成员计数
	err = queries.IncrementChatroomMemberCount(c.Request.Context(), roomId)
	if err != nil {
		// 记录日志但不影响响应
		c.Error(err)
	}

	// 重新查询聊天室信息以获取更新后的成员数
	updatedChatroom, err := queries.GetChatroomByID(c.Request.Context(), roomId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询聊天室信息失败",
			"error":   err.Error(),
		})
		return
	}

	// 转换聊天室类型为字符串
	var roomTypeStr string
	switch updatedChatroom.RoomType {
	case sqlcdb.ChatroomTypePublic:
		roomTypeStr = "public"
	case sqlcdb.ChatroomTypePrivateInviteOnly:
		roomTypeStr = "private"
	case sqlcdb.ChatroomTypePrivatePassword:
		roomTypeStr = "protected"
	default:
		roomTypeStr = "public"
	}

	// 构建响应
	chatroomInfo := ChatroomInfoResponse{
		RoomId:      updatedChatroom.RoomID,
		Name:        updatedChatroom.RoomName,
		Description: updatedChatroom.Description.String,
		Icon:        updatedChatroom.IconUrl.String,
		Type:        roomTypeStr,
		OnlineCount: updatedChatroom.OnlineCount,
		PeopleCount: updatedChatroom.MemberCount,
		CreatedTime: updatedChatroom.CreatedAt,
		LastMessageTime: func() time.Time {
			if updatedChatroom.LastActiveAt.Valid {
				return updatedChatroom.LastActiveAt.Time
			}
			return updatedChatroom.CreatedAt
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
		"message": "加入成功",
		"data": JoinChatRoomResponse{
			Chatroom:   chatroomInfo,
			MemberInfo: memberInfo,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
