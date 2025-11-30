package member

import (
	sqlcdb "chatroombackend/db"
	"chatroombackend/middleware"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SearchMemberItem struct {
	UserId       string `json:"userId"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	Avatar       string `json:"avatar"`
	OnlineStatus string `json:"onlineStatus"`
}

type SearchMemberResponse struct {
	Users    []SearchMemberItem `json:"users"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

// HandleSearchRoomMembers 在聊天室内搜索成员
func HandleSearchRoomMembers(c *gin.Context) {
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

	// 获取搜索关键词
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "搜索关键词不能为空",
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
			"message": "您不是该聊天室成员，无法搜索成员",
		})
		return
	}

	// 获取搜索结果总数
	total, err := queries.CountSearchChatroomMembers(c.Request.Context(), sqlcdb.CountSearchChatroomMembersParams{
		RoomID:  roomId,
		Column2: sql.NullString{String: keyword, Valid: true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取搜索结果数量失败",
			"error":   err.Error(),
		})
		return
	}

	// 搜索成员
	members, err := queries.SearchChatroomMembers(c.Request.Context(), sqlcdb.SearchChatroomMembersParams{
		RoomID:  roomId,
		Column2: sql.NullString{String: keyword, Valid: true},
		Limit:   int64(pageSize),
		Offset:  int64(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索成员失败",
			"error":   err.Error(),
		})
		return
	}

	// 构建响应
	userList := make([]SearchMemberItem, 0, len(members))
	for _, m := range members {
		item := SearchMemberItem{
			UserId:       m.UserID,
			Username:     m.Username,
			Nickname:     m.Nickname.String,
			Avatar:       m.AvatarUrl.String,
			OnlineStatus: string(m.OnlineStatus.UserOnlineStatus),
		}
		userList = append(userList, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": SearchMemberResponse{
			Users:    userList,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}
