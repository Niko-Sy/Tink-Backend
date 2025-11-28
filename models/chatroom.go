package models

import "time"

type ChatRoom struct {
	RoomId          string    `json:"roomId"`             // 聊天室ID（9位数字）
	Name            string    `json:"name"`               // 名称
	Description     string    `json:"description"`        // 描述
	Icon            string    `json:"icon"`               // 图标
	Type            string    `json:"type"`               // 类型: 'public' | 'private' | 'protected'
	Password        string    `json:"password,omitempty"` // 仅protected类型
	CreatorId       string    `json:"creatorId"`          // 创建者ID
	OnlineCount     int       `json:"onlineCount"`        // 在线人数
	PeopleCount     int       `json:"peopleCount"`        // 总人数
	CreatedTime     time.Time `json:"createdTime"`        // 创建时间
	LastMessageTime time.Time `json:"lastMessageTime"`    // 最后消息时间
	Unread          int       `json:"unread,omitempty"`   // 未读消息数（仅客户端）
}

func (cr *ChatRoom) NewChatRoom(name string, description string, icon string, roomType string, password string, creatorId string) *ChatRoom {
	return &ChatRoom{
		Name:            name,
		Description:     description,
		Icon:            icon,
		Type:            roomType,
		Password:        password,
		CreatorId:       creatorId,
		CreatedTime:     time.Now(),
		LastMessageTime: time.Now(),
	}
}
