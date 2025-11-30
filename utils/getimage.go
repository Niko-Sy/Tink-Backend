package utils

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ServeStaticImages 注册静态图片服务路由
func ServeStaticImages(router *gin.Engine, urlPrefix string, uploadDir string) {
	// 确保目录存在
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic("创建图片目录失败: " + err.Error())
	}

	// 静态文件服务
	router.Static(urlPrefix, uploadDir)
}

// GetImageHandler 获取图片的 Handler（带安全检查）
func GetImageHandler(uploadDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取文件路径参数
		filePath := c.Param("filepath")
		if filePath == "" {
			filePath = c.Param("filename")
		}

		if filePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "文件路径不能为空",
			})
			return
		}

		// 安全检查：防止路径遍历攻击
		if strings.Contains(filePath, "..") {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "非法的文件路径",
			})
			return
		}

		// 构建完整路径
		fullPath := filepath.Join(uploadDir, filePath)

		// 检查文件是否存在
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "文件不存在",
			})
			return
		}

		// 返回文件
		c.File(fullPath)
	}
}

// GetAvatarHandler 获取用户头像的 Handler
func GetAvatarHandler(c *gin.Context) {
	fileName := c.Param("filename")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "filename required",
		})
		return
	}

	// 安全检查
	if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid filename",
		})
		return
	}

	uploader := GetImageUploader()
	config := uploader.GetConfig()

	// 头像存储在 avatars 子目录
	filePath := filepath.Join(config.UploadDir, "avatars", fileName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 返回默认头像（重定向）
		c.Redirect(http.StatusFound, config.DefaultAvatar)
		return
	}

	c.File(filePath)
}

// GetChatImageHandler 获取聊天图片的 Handler
func GetChatImageHandler(c *gin.Context) {
	roomID := c.Param("roomid")
	fileName := c.Param("filename")

	if roomID == "" || fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "roomId and filename required",
		})
		return
	}

	// 安全检查
	if strings.Contains(roomID, "..") || strings.Contains(fileName, "..") ||
		strings.Contains(roomID, "/") || strings.Contains(fileName, "/") ||
		strings.Contains(roomID, "\\") || strings.Contains(fileName, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid path",
		})
		return
	}

	uploader := GetImageUploader()
	config := uploader.GetConfig()

	filePath := filepath.Join(config.UploadDir, "chat", roomID, fileName)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "file not found",
		})
		return
	}

	c.File(filePath)
}
