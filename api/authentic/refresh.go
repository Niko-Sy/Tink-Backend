package authentic

import (
	"chatroombackend/middleware"
	"github.com/gin-gonic/gin"
	"strings"
)

func HandleRefresh(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	claims, e := middleware.ParseTokenPurpose(raw, "refresh")
	if e != nil {
		c.JSON(401, gin.H{"code": 401, "businessCode": "UNAUTHORIZED", "message": "刷新凭据无效，请重新登录"})
		return
	}
	q, e := middleware.GetQueriesFromContext(c)
	if e != nil {
		c.JSON(503, gin.H{"code": 503, "message": "服务不可用"})
		return
	}
	actor, e := q.SocialProfile(c.Request.Context(), claims.UserID)
	if e != nil || actor.AccountStatus != "active" {
		c.JSON(403, gin.H{"code": 403, "businessCode": "ACCOUNT_UNAVAILABLE", "message": "账号不可用"})
		return
	}
	access, e := middleware.GenerateToken(claims.UserID, actor.Username)
	if e != nil {
		c.JSON(500, gin.H{"code": 500, "message": "服务不可用"})
		return
	}
	refresh, e := middleware.GenerateRefreshToken(claims.UserID, actor.Username)
	if e != nil {
		c.JSON(500, gin.H{"code": 500, "message": "服务不可用"})
		return
	}
	c.JSON(200, gin.H{"code": 200, "message": "刷新成功", "data": gin.H{"token": access, "refreshToken": refresh, "expiresIn": middleware.JWTExpireSeconds}})
}
