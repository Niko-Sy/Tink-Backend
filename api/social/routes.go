package social

import (
	"chatroombackend/middleware"
	service "chatroombackend/service/social"
	"errors"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
	"time"
)

func WriteError(c *gin.Context, err error) {
	if c.Request.Context().Err() != nil {
		c.AbortWithStatus(499)
		return
	}
	var e *service.Error
	if !errors.As(err, &e) {
		log.Printf("social failure route=%s errorType=%T", c.FullPath(), err)
		e = service.Failure(500, "INTERNAL_ERROR", "服务暂不可用，请重试", nil)
	}
	if e.Status == 429 {
		if details, ok := e.Details.(map[string]any); ok {
			if seconds, ok := details["retryAfterSeconds"].(int); ok {
				c.Header("Retry-After", strconv.Itoa(seconds))
			}
		}
	}
	c.Set("businessCode", e.Code)
	c.AbortWithStatusJSON(e.Status, gin.H{"code": e.Status, "message": e.Message, "businessCode": e.Code, "data": e.Details, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
func success(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"code": 200, "message": "操作成功", "data": data, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
}
func pagination(c *gin.Context) (int, int, error) {
	page, size := 1, 20
	var err error
	if raw, ok := c.GetQuery("page"); ok {
		page, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, service.Failure(400, "INVALID_ARGUMENT", "分页参数无效", nil)
		}
	}
	if raw, ok := c.GetQuery("pageSize"); ok {
		size, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, service.Failure(400, "INVALID_ARGUMENT", "分页参数无效", nil)
		}
	}
	if page < 1 || size < 1 || size > 100 || page > 1000000 {
		return 0, 0, service.Failure(400, "INVALID_ARGUMENT", "分页参数无效", nil)
	}
	return page, size, nil
}
func PublicProfile(s *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := s.Public(c.Request.Context(), c.Param("userid"))
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, p)
	}
}
func Register(v1 *gin.RouterGroup, s *service.Service) {
	auth := v1.Group("")
	auth.Use(middleware.JWTAuthMiddleware(), func(c *gin.Context) {
		if err := s.Actor(c.Request.Context(), c.GetString("userId")); err != nil {
			WriteError(c, err)
			return
		}
		c.Next()
	})
	auth.GET("/users/me/social-capabilities", func(c *gin.Context) { success(c, 200, s.Capabilities) })
	auth.GET("/users/me/social-summary", func(c *gin.Context) {
		data, err := s.Summary(c.Request.Context(), c.GetString("userId"))
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	friends := auth.Group("")
	friends.Use(func(c *gin.Context) {
		if err := s.Enabled(); err != nil {
			WriteError(c, err)
			return
		}
		c.Next()
	})
	friends.GET("/users/search", func(c *gin.Context) {
		data, err := s.Search(c.Request.Context(), c.GetString("userId"), c.Query("userId"), c.Query("username"))
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	auth.GET("/users/:userid/relationship", func(c *gin.Context) {
		data, err := s.Relationship(c.Request.Context(), c.GetString("userId"), c.Param("userid"))
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	friends.POST("/friends/requests", func(c *gin.Context) {
		var req struct {
			ToUserID string `json:"toUserId" binding:"required"`
			Message  string `json:"message"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			WriteError(c, service.Failure(400, "INVALID_ARGUMENT", "申请内容无效", nil))
			return
		}
		data, err := s.CreateRequest(c.Request.Context(), c.GetString("userId"), req.ToUserID, req.Message)
		if err != nil {
			WriteError(c, err)
			return
		}
		status := 200
		if data.Created {
			status = http.StatusCreated
		}
		success(c, status, data)
	})
	friends.GET("/users/me/friend-requests", func(c *gin.Context) {
		page, size, err := pagination(c)
		if err != nil {
			WriteError(c, err)
			return
		}
		data, err := s.Requests(c.Request.Context(), c.GetString("userId"), c.DefaultQuery("direction", "received"), c.Query("status"), page, size)
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	for _, action := range []string{"accept", "reject", "cancel"} {
		friends.POST("/friends/requests/:requestid/"+action, func(c *gin.Context) {
			data, err := s.HandleRequest(c.Request.Context(), c.GetString("userId"), c.Param("requestid"), action)
			if err != nil {
				WriteError(c, err)
				return
			}
			success(c, 200, data)
		})
	}
	friends.GET("/users/me/friends", func(c *gin.Context) {
		page, size, err := pagination(c)
		if err != nil {
			WriteError(c, err)
			return
		}
		data, err := s.Friends(c.Request.Context(), c.GetString("userId"), c.Query("q"), c.DefaultQuery("status", "all"), page, size)
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	friends.GET("/friends/:userid", func(c *gin.Context) {
		data, err := s.Contact(c.Request.Context(), c.GetString("userId"), c.Param("userid"))
		if err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, data)
	})
	friends.DELETE("/friends/:userid", func(c *gin.Context) {
		if err := s.Delete(c.Request.Context(), c.GetString("userId"), c.Param("userid")); err != nil {
			WriteError(c, err)
			return
		}
		success(c, 200, gin.H{"removed": true})
	})
}
