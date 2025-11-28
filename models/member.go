package models

import "time"

type ChatRoomMember struct {
	MemberId      string    `json:"memberId"`      // 成员ID
	RoomId        string    `json:"roomId"`        // 聊天室ID (外键)
	UserId        string    `json:"userId"`        // 用户ID (外键)
	RoomRole      string    `json:"roomRole"`      // 角色: 'owner' | 'admin' | 'member'
	IsMuted       bool      `json:"isMuted"`       // 是否被禁言
	MuteUntil     time.Time `json:"muteUntil,omitempty"` // 禁言结束时间
	JoinedAt      time.Time `json:"joinedAt"`      // 加入时间
	LastReadAt    time.Time `json:"lastReadAt,omitempty"` // 最后阅读时间
	IsActive      bool      `json:"isActive"`      // 是否活跃
	LeftAt        time.Time `json:"leftAt,omitempty"` // 离开时间
}
