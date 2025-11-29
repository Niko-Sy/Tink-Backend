package chatroom

import (
	"chatroombackend/models"
	"chatroombackend/utils"
	"log"
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

func HandleCreateRoom(c *gin.Context) {
	var req CreateChatRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从上下文中获取当前用户ID（实际实现中需要从JWT token解析）
	currentUserID := c.GetString("userId")
	if currentUserID == "" {
		currentUserID = "U123456789" // 测试用
	}

	// 创建聊天室
	chatRoom := models.ChatRoom{
		RoomId:          utils.GenerateChatRoomID(), // 需要实现生成房间ID的函数
		Name:            req.Name,
		Description:     req.Description,
		Icon:            req.Icon,
		Type:            req.Type,
		Password:        req.Password,
		CreatorId:       currentUserID,
		OnlineCount:     1,
		PeopleCount:     1,
		CreatedTime:     time.Now(),
		LastMessageTime: time.Now(),
	}

	// TODO: 将聊天室保存到数据库

	// 创建房主成员记录
	member := models.ChatRoomMember{
		MemberId: utils.GenerateMemberID(chatRoom.RoomId, currentUserID), // 需要实现生成成员ID的函数
		RoomId:   chatRoom.RoomId,
		UserId:   currentUserID,
		RoomRole: "owner",
		IsMuted:  false,
		JoinedAt: time.Now(),
		IsActive: true,
	}

	log.Println("member:", member)

	// TODO: 将成员记录保存到数据库

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建成功",
		"data":    chatRoom,
	})
}
