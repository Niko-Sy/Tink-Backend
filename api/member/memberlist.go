package member

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// 成员信息响应
type MemberUserInfo struct {
	UserId   string `json:"userId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Status   string `json:"status"`
}

type MemberDetailInfo struct {
	MemberId  string     `json:"memberId"`
	RoomRole  string     `json:"roomRole"`
	IsMuted   bool       `json:"isMuted"`
	MuteUntil *time.Time `json:"muteUntil"`
	JoinedAt  time.Time  `json:"joinedAt"`
	IsActive  bool       `json:"isActive"`
}

type MemberListItem struct {
	UserId     string           `json:"userId"`
	Username   string           `json:"username"`
	Nickname   string           `json:"nickname"`
	Name       string           `json:"name"`
	Avatar     string           `json:"avatar"`
	Status     string           `json:"status"`
	MemberInfo MemberDetailInfo `json:"memberInfo"`
}

type MemberListResponse struct {
	Members     []MemberListItem `json:"members"`
	Total       int64            `json:"total"`
	OnlineCount int64            `json:"onlineCount"`
	Page        int              `json:"page"`
	PageSize    int              `json:"pageSize"`
}

// HandleListRoomMembers 获取聊天室成员列表
func HandleListRoomMembers(c *gin.Context) {
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

	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	statusFilter := c.DefaultQuery("status", "all")

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

	// 检查当前用户是否是聊天室成员
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
	if !isInRoom {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "您不是该聊天室成员，无法查看成员列表",
		})
		return
	}

	// 获取成员总数
	total, err := queries.CountChatroomMembers(c.Request.Context(), roomId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取成员数量失败",
			"error":   err.Error(),
		})
		return
	}

	// 获取成员列表
	members, err := queries.GetChatroomMembers(c.Request.Context(), sqlcdb.GetChatroomMembersParams{
		RoomID: roomId,
		Limit:  int64(pageSize),
		Offset: int64(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取成员列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应并统计在线人数
	memberList := make([]MemberListItem, 0, len(members))
	onlineCount := int64(0)

	for _, m := range members {
		status := string(m.OnlineStatus.UserOnlineStatus)

		// 根据状态过滤
		if statusFilter != "all" {
			if statusFilter == "online" && status != "online" {
				continue
			}
			if statusFilter == "away" && status != "away" {
				continue
			}
			if statusFilter == "offline" && status != "offline" {
				continue
			}
		}

		// 统计在线人数
		if status == "online" {
			onlineCount++
		}

		item := MemberListItem{
			UserId:   m.UserID,
			Username: m.Username,
			Nickname: m.Nickname.String,
			Name:     m.Nickname.String,
			Avatar:   m.AvatarUrl.String,
			Status:   status,
			MemberInfo: MemberDetailInfo{
				MemberId: m.MemberRelID,
				RoomRole: string(m.MemberRole),
				IsMuted:  m.MuteStatus == sqlcdb.MemberMuteStatusMuted,
				MuteUntil: func() *time.Time {
					if m.MuteExpiresAt.Valid {
						return &m.MuteExpiresAt.Time
					}
					return nil
				}(),
				JoinedAt: m.JoinedAt,
				IsActive: m.IsActive,
			},
		}
		memberList = append(memberList, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": MemberListResponse{
			Members:     memberList,
			Total:       total,
			OnlineCount: onlineCount,
			Page:        page,
			PageSize:    pageSize,
		},
	})
}
