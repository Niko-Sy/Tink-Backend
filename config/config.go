package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	Database  DatabaseConfig  `yaml:"database"`
	Server    ServerConfig    `yaml:"server"`
	Upload    UploadConfig    `yaml:"upload"`
	JWT       JWTConfig       `yaml:"jwt"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	Logger    LoggerConfig    `yaml:"logger"`
	Social    SocialConfig    `yaml:"social"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	URL             string `yaml:"url"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // 秒
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int      `yaml:"port"`
	Host           string   `yaml:"host"`
	BaseURL        string   `yaml:"base_url"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type SocialConfig struct {
	Friends         bool `yaml:"friends"`
	DirectMessages  bool `yaml:"direct_messages"`
	RoomInvitations bool `yaml:"room_invitations"`
}

// UploadConfig 上传配置
type UploadConfig struct {
	UploadDir     string   `yaml:"upload_dir"`
	MaxSize       int64    `yaml:"max_size"`
	AllowedTypes  []string `yaml:"allowed_types"`
	URLPrefix     string   `yaml:"url_prefix"`
	DefaultAvatar string   `yaml:"default_avatar"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret            string `yaml:"secret"`
	Expiration        int    `yaml:"expiration"`         // 秒
	RefreshExpiration int    `yaml:"refresh_expiration"` // 秒
}

// WebSocketConfig WebSocket 配置
type WebSocketConfig struct {
	ReadBufferSize  int `yaml:"read_buffer_size"`
	WriteBufferSize int `yaml:"write_buffer_size"`
	PingInterval    int `yaml:"ping_interval"` // 秒
	PongTimeout     int `yaml:"pong_timeout"`  // 秒
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	LogDir        string `yaml:"log_dir"`
	LogLevel      string `yaml:"log_level"`
	EnableConsole bool   `yaml:"enable_console"`
	EnableFile    bool   `yaml:"enable_file"`
	MaxAge        int    `yaml:"max_age"` // 天
}

var globalConfig *Config

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	cfg := Config{Social: SocialConfig{Friends: true, DirectMessages: true, RoomInvitations: true}, JWT: JWTConfig{Expiration: 86400, RefreshExpiration: 604800}}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

// GetDatabaseURL 获取数据库连接字符串
func (c *Config) GetDatabaseURL() string {
	return c.Database.URL
}

// GetServerAddress 获取服务器监听地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
