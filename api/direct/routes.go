package direct

import (
	socialapi "chatroombackend/api/social"
	"chatroombackend/middleware"
	direct "chatroombackend/service/direct"
	social "chatroombackend/service/social"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func reply(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"code": 200, "message": "操作成功", "data": data, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
func limit(c *gin.Context, defaultLimit int) (int, error) {
	v, err := strconv.Atoi(c.DefaultQuery("pageSize", strconv.Itoa(defaultLimit)))
	if err != nil || v < 1 || v > 100 {
		return 0, social.Failure(400, "INVALID_ARGUMENT", "分页参数无效", nil)
	}
	return v, nil
}
func seq(c *gin.Context, key string) (int64, error) {
	raw := c.DefaultQuery(key, "0")
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0, social.Failure(400, "INVALID_MESSAGE_CURSOR", "消息游标无效", nil)
	}
	return v, nil
}
func Register(v1 *gin.RouterGroup, s *direct.Service, actor *social.Service) {
	auth := v1.Group("")
	auth.Use(middleware.JWTAuthMiddleware(), func(c *gin.Context) {
		if err := actor.Actor(c.Request.Context(), c.GetString("userId")); err != nil {
			socialapi.WriteError(c, err)
			return
		}
		c.Next()
	})
	auth.POST("/direct-conversations", func(c *gin.Context) {
		var input struct {
			Peer string `json:"peerUserId" binding:"required"`
		}
		if c.ShouldBindJSON(&input) != nil {
			socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "私信对象无效", nil))
			return
		}
		out, err := s.Create(c.Request.Context(), c.GetString("userId"), input.Peer)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		status := 200
		if out.Created {
			status = 201
		}
		reply(c, status, out)
	})
	auth.GET("/users/me/direct-conversations", func(c *gin.Context) {
		size, err := limit(c, 20)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		out, err := s.List(c.Request.Context(), c.GetString("userId"), c.Query("cursor"), size)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.GET("/direct-conversations/:roomid", func(c *gin.Context) {
		out, err := s.Detail(c.Request.Context(), c.GetString("userId"), c.Param("roomid"))
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.GET("/direct-conversations/:roomid/messages", func(c *gin.Context) {
		size, err := limit(c, 50)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		before, err := seq(c, "beforeSeq")
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		after, err := seq(c, "afterSeq")
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		_, useAfter := c.GetQuery("afterSeq")
		_, useBefore := c.GetQuery("beforeSeq")
		if useAfter && useBefore {
			socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "不能同时指定两个消息游标", nil))
			return
		}
		out, err := s.History(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), before, after, useAfter, size)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.GET("/direct-conversations/:roomid/changes", func(c *gin.Context) {
		size, err := limit(c, 50)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		revision, err := seq(c, "afterRevision")
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		out, err := s.Changes(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), revision, size)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.POST("/direct-conversations/:roomid/messages", func(c *gin.Context) {
		var input direct.SendInput
		if c.ShouldBindJSON(&input) != nil {
			socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "消息无效", nil))
			return
		}
		out, err := s.Send(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), input)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.POST("/direct-conversations/:roomid/messages/:messageid/edit", func(c *gin.Context) {
		var input struct {
			Text     string `json:"text"`
			Expected string `json:"expectedRevision"`
		}
		if c.ShouldBindJSON(&input) != nil {
			socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "消息无效", nil))
			return
		}
		out, err := s.Mutate(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), c.Param("messageid"), input.Text, input.Expected, false)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.POST("/direct-conversations/:roomid/messages/:messageid/delete", func(c *gin.Context) {
		out, err := s.Mutate(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), c.Param("messageid"), "", "", true)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
	auth.POST("/direct-conversations/:roomid/read", func(c *gin.Context) {
		var input struct {
			ID string `json:"lastReadMessageId" binding:"required"`
		}
		if c.ShouldBindJSON(&input) != nil {
			socialapi.WriteError(c, social.Failure(400, "INVALID_ARGUMENT", "阅读目标无效", nil))
			return
		}
		out, err := s.Read(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), input.ID)
		if err != nil {
			socialapi.WriteError(c, err)
			return
		}
		reply(c, 200, out)
	})
}
