package models

import "time"

type Notification struct {
	notificationId string
	receiverId     string
	noteType       string
	title          string
	content        string
	data           map[string]interface{}
	isRead         bool
	createdAt      time.Time
}
