package middleware

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

// GroupConversationGuard also handles legacy room IDs carried in JSON bodies.
func GroupConversationGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomid")
		if roomID == "" && c.Request.Method == http.MethodPost && c.Request.Body != nil {
			body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
			if err != nil {
				c.AbortWithStatusJSON(400, gin.H{"code": 400, "message": "请求无效"})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			var fields map[string]json.RawMessage
			if json.Unmarshal(body, &fields) == nil {
				if _, ok := fields["conversation_kind"]; ok {
					c.AbortWithStatusJSON(400, gin.H{"code": 400, "message": "不能指定会话种类"})
					return
				}
				if _, ok := fields["conversationKind"]; ok {
					c.AbortWithStatusJSON(400, gin.H{"code": 400, "message": "不能指定会话种类"})
					return
				}
				_ = json.Unmarshal(fields["roomId"], &roomID)
			}
		}
		if roomID != "" {
			q, err := GetQueriesFromContext(c)
			if err != nil {
				c.AbortWithStatusJSON(500, gin.H{"code": 500, "message": "服务不可用"})
				return
			}
			kind, err := q.ConversationKind(c.Request.Context(), roomID)
			if errors.Is(err, sql.ErrNoRows) || err == nil && kind != "group" {
				c.AbortWithStatusJSON(404, gin.H{"code": 404, "businessCode": "RESOURCE_NOT_FOUND", "message": "聊天室不可访问"})
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(500, gin.H{"code": 500, "message": "服务不可用"})
				return
			}
		}
		c.Next()
	}
}
