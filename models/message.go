package models

import "time"

type Message struct {
	MessageId       string    `json:"messageId"`
	RoomId          string    `json:"roomId"`
	UserId          string    `json:"userId"`
	MessageType     string    `json:"messageType"`
	Content         string    `json:"content"`
	Time            time.Time `json:"time"`
	FileUrl         string    `json:"fileUrl"`
	IsEdited        bool      `json:"isEdited"`
	EditedAt        time.Time `json:"editedAt"`
	ImportmessageId string    `json:"importmessageId"`
}
