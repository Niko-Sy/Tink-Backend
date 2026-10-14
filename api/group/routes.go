package group

import (
	"chatroombackend/api/social"
	svc "chatroombackend/service/group"
	s "chatroombackend/service/social"
	"github.com/gin-gonic/gin"
)

func reply(c *gin.Context, v any, e error) {
	if e != nil {
		social.WriteError(c, e)
		return
	}
	c.JSON(200, gin.H{"code": 200, "message": "操作成功", "data": v})
}
func Register(auth *gin.RouterGroup, g *svc.Service) {
	auth.POST("/createroom", func(c *gin.Context) {
		var in svc.CreateInput
		if c.ShouldBindJSON(&in) != nil {
			reply(c, nil, s.Failure(400, "INVALID_ARGUMENT", "参数无效", nil))
			return
		}
		v, e := g.Create(c.Request.Context(), c.GetString("userId"), in)
		reply(c, v, e)
	})
	auth.POST("/joinroom", func(c *gin.Context) {
		var in struct {
			RoomID   string `json:"roomId" binding:"required"`
			Password string `json:"password"`
		}
		if c.ShouldBindJSON(&in) != nil {
			reply(c, nil, s.Failure(400, "INVALID_ARGUMENT", "参数无效", nil))
			return
		}
		v, e := g.Join(c.Request.Context(), c.GetString("userId"), in.RoomID, in.Password)
		reply(c, v, e)
	})
	auth.POST("/leaveroom", func(c *gin.Context) {
		var in struct {
			RoomID string `json:"roomId" binding:"required"`
		}
		if c.ShouldBindJSON(&in) != nil {
			reply(c, nil, s.Failure(400, "INVALID_ARGUMENT", "参数无效", nil))
			return
		}
		reply(c, gin.H{"left": true}, g.Leave(c.Request.Context(), c.GetString("userId"), in.RoomID))
	})
	auth.POST("/:roomid/delete", func(c *gin.Context) {
		reply(c, gin.H{"deleted": true}, g.Delete(c.Request.Context(), c.GetString("userId"), c.Param("roomid")))
	})
	auth.POST("/:roomid/update", func(c *gin.Context) {
		var in svc.Update
		if c.ShouldBindJSON(&in) != nil {
			reply(c, nil, s.Failure(400, "INVALID_ARGUMENT", "参数无效", nil))
			return
		}
		v, e := g.Update(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), in)
		reply(c, v, e)
	})
	for _, action := range []string{"kick", "setadmin", "removeadmin"} {
		auth.POST("/:roomid/members/"+action, func(c *gin.Context) {
			var in struct {
				MemberID string `json:"memberid" binding:"required"`
				Reason   string `json:"reason"`
			}
			if c.ShouldBindJSON(&in) != nil {
				reply(c, nil, s.Failure(400, "INVALID_ARGUMENT", "参数无效", nil))
				return
			}
			reply(c, gin.H{"updated": true}, g.MemberAction(c.Request.Context(), c.GetString("userId"), c.Param("roomid"), in.MemberID, action, in.Reason))
		})
	}
}
