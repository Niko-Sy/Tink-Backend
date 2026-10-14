package invitation

import (
	socialapi "chatroombackend/api/social"
	"chatroombackend/middleware"
	svc "chatroombackend/service/invitation"
	social "chatroombackend/service/social"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func page(c *gin.Context) (int, int, error) {
	p, s := 1, 20
	var e error
	if v, ok := c.GetQuery("page"); ok {
		p, e = strconv.Atoi(v)
		if e != nil {
			return 0, 0, social.Failure(400, "INVALID_ARGUMENT", "分页无效", nil)
		}
	}
	if v, ok := c.GetQuery("pageSize"); ok {
		s, e = strconv.Atoi(v)
		if e != nil {
			return 0, 0, social.Failure(400, "INVALID_ARGUMENT", "分页无效", nil)
		}
	}
	if p < 1 || p > 1000000 || s < 1 || s > 100 {
		return 0, 0, social.Failure(400, "INVALID_ARGUMENT", "分页无效", nil)
	}
	return p, s, nil
}
func reply(c *gin.Context, status int, data any, e error) {
	if e != nil {
		socialapi.WriteError(c, e)
		return
	}
	c.JSON(status, gin.H{"code": 200, "message": "操作成功", "data": data, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
func Register(v1 *gin.RouterGroup, s *svc.Service) {
	auth := v1.Group("")
	auth.Use(middleware.JWTAuthMiddleware())
	auth.GET("/users/me/invitable-chatrooms", func(c *gin.Context) {
		p, n, e := page(c)
		if e != nil {
			reply(c, 200, nil, e)
			return
		}
		v, e := s.Rooms(c.Request.Context(), c.GetString("userId"), c.Query("inviteeId"), c.Query("q"), p, n)
		reply(c, 200, v, e)
	})
	auth.GET("/chatroom/:roomid/invite-candidates", func(c *gin.Context) {
		p, n, e := page(c)
		if e != nil {
			reply(c, 200, nil, e)
			return
		}
		v, e := s.Candidates(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), c.Query("q"), p, n)
		reply(c, 200, v, e)
	})
	auth.POST("/chatroom/:roomid/invitations", func(c *gin.Context) {
		var in struct {
			InviteeID string `json:"inviteeId" binding:"required"`
			Message   string `json:"message"`
		}
		if c.ShouldBindJSON(&in) != nil {
			reply(c, 200, nil, social.Failure(400, "INVALID_ARGUMENT", "邀请参数无效", nil))
			return
		}
		v, e := s.Create(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), in.InviteeID, in.Message)
		status := 200
		if v.Created {
			status = 201
		}
		reply(c, status, v, e)
	})
	for _, scope := range []string{"received", "room"} {
		path := "/users/me/chatroom-invitations"
		if scope == "room" {
			path = "/chatroom/:roomid/invitations"
		}
		auth.GET(path, func(c *gin.Context) {
			p, n, e := page(c)
			if e != nil {
				reply(c, 200, nil, e)
				return
			}
			direction := scope
			if scope != "room" {
				direction = c.DefaultQuery("direction", "received")
			}
			v, e := s.List(c.Request.Context(), c.GetString("userId"), direction, c.Param("roomid"), c.Query("status"), p, n)
			reply(c, 200, v, e)
		})
	}
	for _, action := range []string{"accept", "reject", "cancel"} {
		auth.POST("/chatroom-invitations/:invitationid/"+action, func(c *gin.Context) {
			v, e := s.Handle(c.Request.Context(), c.GetString("userId"), c.Param("invitationid"), action)
			reply(c, 200, v, e)
		})
	}
}
