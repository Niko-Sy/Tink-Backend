package messages

import (
	socialapi "chatroombackend/api/social"
	"chatroombackend/service/direct"
	core "chatroombackend/service/message"
	social "chatroombackend/service/social"
	"github.com/gin-gonic/gin"
	"time"
)

func HandleSendMessage(c *gin.Context) {
	var input struct {
		Type   string `json:"type"`
		Text   string `json:"text"`
		Quoted string `json:"quotedMessageId"`
		Reply  string `json:"replyToMessageId"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Text == "" {
		socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "消息无效", nil))
		return
	}
	if input.Quoted == "" {
		input.Quoted = input.Reply
	}
	service := c.MustGet("messageService").(*core.Service)
	out, err := service.Send(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), direct.SendInput{Type: input.Type, Text: input.Text, QuotedMessageID: input.Quoted})
	if err != nil {
		socialapi.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 200, "message": "发送成功", "data": out, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
