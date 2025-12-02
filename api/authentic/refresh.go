package authentic

import (
	"chatroombackend/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleRefresh(c *gin.Context) {
	refreshToken := c.Query("refresh_Token")
	if refreshToken == "" {
		refreshToken = c.PostForm("refresh_Token")
	}
	if refreshToken == "" {
		refreshToken = c.GetHeader("Authorization")
		if len(refreshToken) > 7 && refreshToken[:7] == "Bearer " {
			refreshToken = refreshToken[7:]
		}
	}
	if refreshToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "缺少refresh_Token",
		})
		return
	}
	claims, err := middleware.ParseToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "refresh_Token错误",
			"error":   err.Error(),
		})
		return
	}
	_, err = middleware.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成access_Token错误",
			"error":   err.Error(),
		})
		return
	}
	newRefreshToken, err := middleware.GenerateRefreshToken(claims.UserID, claims.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成refresh_Token错误",
			"error":   err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "刷新成功",
		"data": gin.H{
			"token":     newRefreshToken,
			"expiresIn": 86400,
		},
	})
}
