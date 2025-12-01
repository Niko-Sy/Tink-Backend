package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ImageConfig 图片上传配置
type ImageConfig struct {
	UploadDir     string   // 上传目录
	MaxSize       int64    // 最大文件大小（字节）
	AllowedTypes  []string // 允许的文件类型
	URLPrefix     string   // URL 前缀
	BaseURL       string   // 服务器基础URL，例如 "http://localhost:8080"
	DefaultAvatar string   // 默认头像
}

// DefaultImageConfig 默认配置
var DefaultImageConfig = ImageConfig{
	UploadDir:     "./uploads",
	MaxSize:       5 * 1024 * 1024, // 5MB
	AllowedTypes:  []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
	URLPrefix:     "/static/images",
	BaseURL:       "http://localhost:8080",
	DefaultAvatar: "https://example.com/default-avatar.jpg",
}

// ImageUploader 图片上传器
type ImageUploader struct {
	config ImageConfig
}

// NewImageUploader 创建图片上传器
func NewImageUploader(config *ImageConfig) *ImageUploader {
	if config == nil {
		config = &DefaultImageConfig
	}
	// 确保上传目录存在
	if err := os.MkdirAll(config.UploadDir, 0755); err != nil {
		panic(fmt.Sprintf("创建上传目录失败: %v", err))
	}
	return &ImageUploader{config: *config}
}

// UploadResult 上传结果
type UploadResult struct {
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	FileURL     string `json:"fileUrl"`
	FileSize    int64  `json:"fileSize"`
	ContentType string `json:"contentType"`
}

// ValidateImage 验证图片文件
func (u *ImageUploader) ValidateImage(file *multipart.FileHeader) error {
	// 检查文件大小
	if file.Size > u.config.MaxSize {
		return fmt.Errorf("文件大小超过限制，最大允许 %d MB", u.config.MaxSize/(1024*1024))
	}

	// 检查文件类型
	contentType := file.Header.Get("Content-Type")
	if !u.isAllowedType(contentType) {
		return fmt.Errorf("不支持的文件类型: %s，允许的类型: %v", contentType, u.config.AllowedTypes)
	}

	return nil
}

// isAllowedType 检查是否为允许的文件类型
func (u *ImageUploader) isAllowedType(contentType string) bool {
	for _, t := range u.config.AllowedTypes {
		if t == contentType {
			return true
		}
	}
	return false
}

// GenerateFileName 生成唯一文件名
func (u *ImageUploader) GenerateFileName(originalName string, userID string) string {
	ext := filepath.Ext(originalName)
	// 使用时间戳 + 用户ID + 随机串生成唯一文件名
	hash := md5.Sum([]byte(fmt.Sprintf("%s_%s_%d", userID, originalName, time.Now().UnixNano())))
	return fmt.Sprintf("%s_%s%s", userID, hex.EncodeToString(hash[:8]), ext)
}

// SaveFile 保存文件
func (u *ImageUploader) SaveFile(file *multipart.FileHeader, fileName string) (*UploadResult, error) {
	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer src.Close()

	// 创建目标文件
	filePath := filepath.Join(u.config.UploadDir, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %v", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %v", err)
	}

	return &UploadResult{
		FileName:    fileName,
		FilePath:    filePath,
		FileURL:     fmt.Sprintf("%s%s/%s", u.config.BaseURL, u.config.URLPrefix, fileName),
		FileSize:    file.Size,
		ContentType: file.Header.Get("Content-Type"),
	}, nil
}

// UploadAvatar 上传头像（完整流程）
func (u *ImageUploader) UploadAvatar(file *multipart.FileHeader, userID string) (*UploadResult, error) {
	// 验证文件
	if err := u.ValidateImage(file); err != nil {
		return nil, err
	}

	// 创建 avatars 子目录
	avatarsDir := filepath.Join(u.config.UploadDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	// 生成文件名（头像使用特定前缀）
	fileName := "avatar_" + u.GenerateFileName(file.Filename, userID)

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer src.Close()

	// 创建目标文件
	filePath := filepath.Join(avatarsDir, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %v", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %v", err)
	}

	return &UploadResult{
		FileName:    fileName,
		FilePath:    filePath,
		FileURL:     fmt.Sprintf("%s%s/avatars/%s", u.config.BaseURL, u.config.URLPrefix, fileName),
		FileSize:    file.Size,
		ContentType: file.Header.Get("Content-Type"),
	}, nil
}

// UploadChatImage 上传聊天图片
func (u *ImageUploader) UploadChatImage(file *multipart.FileHeader, userID string, roomID string) (*UploadResult, error) {
	// 验证文件
	if err := u.ValidateImage(file); err != nil {
		return nil, err
	}

	// 生成文件名（聊天图片存储到 chat/roomID 子目录）
	chatDir := filepath.Join(u.config.UploadDir, "chat", roomID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	fileName := "chat_" + u.GenerateFileName(file.Filename, userID)

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer src.Close()

	// 创建目标文件
	filePath := filepath.Join(chatDir, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %v", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %v", err)
	}

	return &UploadResult{
		FileName:    fileName,
		FilePath:    filePath,
		FileURL:     fmt.Sprintf("%s%s/chat/%s/%s", u.config.BaseURL, u.config.URLPrefix, roomID, fileName),
		FileSize:    file.Size,
		ContentType: file.Header.Get("Content-Type"),
	}, nil
}

// DeleteFile 删除文件
func (u *ImageUploader) DeleteFile(fileName string) error {
	filePath := filepath.Join(u.config.UploadDir, fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在，视为删除成功
	}
	return os.Remove(filePath)
}

// GetFilePath 获取文件完整路径
func (u *ImageUploader) GetFilePath(fileName string) string {
	return filepath.Join(u.config.UploadDir, fileName)
}

// FileExists 检查文件是否存在
func (u *ImageUploader) FileExists(fileName string) bool {
	filePath := filepath.Join(u.config.UploadDir, fileName)
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// GetConfig 获取配置
func (u *ImageUploader) GetConfig() ImageConfig {
	return u.config
}

// 全局上传器实例
var globalUploader *ImageUploader

// InitImageUploader 初始化全局上传器
func InitImageUploader(config *ImageConfig) {
	globalUploader = NewImageUploader(config)
}

// GetImageUploader 获取全局上传器
func GetImageUploader() *ImageUploader {
	if globalUploader == nil {
		globalUploader = NewImageUploader(nil)
	}
	return globalUploader
}

// ====== Gin Handler 辅助函数 ======

// HandleUploadAvatar 处理头像上传的 Gin Handler
func HandleUploadAvatar(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请选择要上传的文件", "error": err.Error()})
		return
	}

	uploader := GetImageUploader()
	result, err := uploader.UploadAvatar(file, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "上传成功",
		"data": gin.H{
			"url":      result.FileURL,
			"fileName": result.FileName,
			"fileSize": result.FileSize,
		},
	})
}

// HandleUploadChatImage 处理聊天图片上传的 Gin Handler
func HandleUploadChatImage(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "unauthorized"})
		return
	}

	roomID := c.Param("roomid")
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "roomId required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请选择要上传的文件", "error": err.Error()})
		return
	}

	uploader := GetImageUploader()
	result, err := uploader.UploadChatImage(file, userID, roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "上传成功",
		"data": gin.H{
			"url":      result.FileURL,
			"fileName": result.FileName,
			"fileSize": result.FileSize,
		},
	})
}

// ServeImage 提供图片访问（用于静态文件服务）
func ServeImage(c *gin.Context) {
	fileName := c.Param("filename")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "filename required"})
		return
	}

	// 安全检查：防止路径遍历攻击
	if strings.Contains(fileName, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid filename"})
		return
	}

	uploader := GetImageUploader()
	filePath := uploader.GetFilePath(fileName)

	if !uploader.FileExists(fileName) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "file not found"})
		return
	}

	c.File(filePath)
}
