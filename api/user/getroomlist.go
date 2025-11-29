package user

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatroomListItem struct {
	RoomId            string                `json:"roomId"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	Icon              string                `json:"icon"`
	Type              string                `json:"type"`
	CreatorId         string                `json:"creatorId"`
	OnlineCount       int32                 `json:"onlineCount"`
	PeopleCount       int32                 `json:"peopleCount"`
	Unread            int64                 `json:"unread"`
	CreatedTime       time.Time             `json:"createdTime"`
	LastMessageTime   time.Time             `json:"lastMessageTime"`
	CurrentUserMember CurrentUserMemberInfo `json:"currentUserMember"`
}

type CurrentUserMemberInfo struct {
	MemberId string    `json:"memberId"`
	RoomRole string    `json:"roomRole"`
	IsMuted  bool      `json:"isMuted"`
	JoinedAt time.Time `json:"joinedAt"`
}

type GetUserChatroomsResponse struct {
	Chatrooms []ChatroomListItem `json:"chatrooms"`
	Total     int64              `json:"total"`
	Page      int                `json:"page"`
	PageSize  int                `json:"pageSize"`
}

func HandleGetUserChatrooms(c *gin.Context) {
	// 从JWT中间件获取用户ID
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录，请先登录获取Token",
		})
		return
	}

	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

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

	// 获取用户聊天室总数
	total, err := queries.CountUserChatrooms(c.Request.Context(), currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取聊天室数量失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取用户聊天室列表
	chatrooms, err := queries.ListUserChatrooms(c.Request.Context(), sqlcdb.ListUserChatroomsParams{
		UserID: currentUserID,
		Limit:  int64(pageSize),
		Offset: int64(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取聊天室列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	chatroomList := make([]ChatroomListItem, 0, len(chatrooms))
	for _, cr := range chatrooms {
		// 获取房主信息
		owner, err := queries.GetChatroomOwner(c.Request.Context(), cr.RoomID)
		creatorId := ""
		if err == nil {
			creatorId = owner.UserID
		}

		// 转换聊天室类型为前端格式
		var roomType string
		switch cr.RoomType {
		case sqlcdb.ChatroomTypePublic:
			roomType = "public"
		case sqlcdb.ChatroomTypePrivateInviteOnly:
			roomType = "private"
		case sqlcdb.ChatroomTypePrivatePassword:
			roomType = "protected"
		default:
			roomType = string(cr.RoomType)
		}

		// 计算未读消息数（简单实现：如果有 last_read_at 则计算，否则为 0）
		// TODO: 实际项目中需要根据 messages 表统计未读数
		unread := int64(0)

		item := ChatroomListItem{
			RoomId:      cr.RoomID,
			Name:        cr.RoomName,
			Description: cr.Description.String,
			Icon:        cr.IconUrl.String,
			Type:        roomType,
			CreatorId:   creatorId,
			OnlineCount: cr.OnlineCount,
			PeopleCount: cr.MemberCount,
			Unread:      unread,
			CreatedTime: cr.CreatedAt,
			LastMessageTime: func() time.Time {
				if cr.LastActiveAt.Valid {
					return cr.LastActiveAt.Time
				}
				return cr.CreatedAt
			}(),
			CurrentUserMember: CurrentUserMemberInfo{
				MemberId: cr.MemberRelID,
				RoomRole: string(cr.MemberRole),
				IsMuted:  cr.MuteStatus == sqlcdb.MemberMuteStatusMuted,
				JoinedAt: cr.JoinedAt,
			},
		}
		chatroomList = append(chatroomList, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": GetUserChatroomsResponse{
			Chatrooms: chatroomList,
			Total:     total,
			Page:      page,
			PageSize:  pageSize,
		},
	})
}
