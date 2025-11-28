package models

import "time"

type FriendRequest struct {
	RequestId  string    `json:"requestId"`  // 请求ID
	SenderId   string    `json:"senderId"`   // 发送者ID
	ReceiverId string    `json:"receiverId"` // 接收者ID
	Status     string    `json:"status"`     // 状态: 'pending' | 'accepted' | 'rejected'
	Message    string    `json:"message"`    // 消息
	CreatedAt  time.Time `json:"createdAt"`  // 创建时间
	HandledAt  time.Time `json:"handledAt"`  // 更新时间
}
