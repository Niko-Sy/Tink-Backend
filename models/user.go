package models

import "time"

type User struct {
	UserId            string    `json:"userId"`                      // 用户ID（U+9位数字）
	Password          string    `json:"password"`                    // 密码（加密存储）
	Username          string    `json:"username"`                    // 用户名（唯一）
	Nickname          string    `json:"nickname,omitempty"`          // 昵称
	Name              string    `json:"name"`                        // 显示名称
	Avatar            string    `json:"avatar"`                      // 头像URL
	Email             string    `json:"email,omitempty"`             // 邮箱
	Phone             string    `json:"phone,omitempty"`             // 手机号
	Signature         string    `json:"signature,omitempty"`         // 个性签名
	OnlineStatus      string    `json:"onlineStatus"`                // 在线状态: 'online' | 'away' | 'busy' | 'offline'
	AccountStatus     string    `json:"accountStatus"`               // 账号状态: 'active' | 'inactive' | 'suspended'
	SystemRole        string    `json:"systemRole"`                  // 系统角色: 'super_admin' | 'user'
	GlobalMuteStatus  string    `json:"globalMuteStatus,omitempty"`  // 全局禁言状态: 'muted' | 'unmuted'
	GlobalMuteEndTime time.Time `json:"globalMuteEndTime,omitempty"` // 全局禁言结束时间
	RegisterTime      time.Time `json:"registerTime"`                // 注册时间
	LastLoginTime     time.Time `json:"lastLoginTime"`               // 最后登录时间
}
