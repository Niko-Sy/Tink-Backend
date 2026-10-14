package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWT 配置
var (
	JWTSecretKey      = []byte("your-secret-key-change-in-production") // 正常启动从 config.yaml 的 jwt.secret 读取
	JWTExpireSeconds  = 86400
	JWTRefreshSeconds = 604800
	JWTIssuer         = "chatroom-backend"
)

// CustomClaims 自定义JWT Claims
type CustomClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Purpose  string `json:"purpose"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT Token
func GenerateToken(userID, username string) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Purpose:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(JWTExpireSeconds) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    JWTIssuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecretKey)
}

// GenerateRefreshToken 生成Refresh Token
func GenerateRefreshToken(userID, username string) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		Username: username,
		Purpose:  "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(JWTRefreshSeconds) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    JWTIssuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecretKey)
}

// ParseToken 解析JWT Token
func ParseToken(raw string) (*CustomClaims, error) { return ParseTokenPurpose(raw, "access") }
func ParseTokenPurpose(raw, purpose string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) { return JWTSecretKey, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(JWTIssuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil {
		return nil, errors.New("无效登录凭据")
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid || claims.Purpose != purpose || claims.UserID == "" || claims.Subject != claims.UserID {
		return nil, errors.New("无效登录凭据")
	}
	return claims, nil
}

// JWTAuthMiddleware JWT认证中间件
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		// 从Header获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未提供认证信息，请在Header中传入Authorization: Bearer <token>",
			})
			c.Abort()
			return
		}

		// 检查Bearer前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "认证格式错误，应为: Bearer <token>",
			})
			c.Abort()
			return
		}

		// 解析token
		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Token无效或已过期",
			})
			c.Abort()
			return
		}

		q, dbErr := GetQueriesFromContext(c)
		if dbErr != nil {
			c.AbortWithStatusJSON(503, gin.H{"code": 503, "message": "服务不可用"})
			return
		}
		actor, dbErr := q.SocialProfile(c.Request.Context(), claims.UserID)
		if dbErr != nil || actor.AccountStatus != "active" {
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "businessCode": "ACCOUNT_UNAVAILABLE", "message": "账号不可用"})
			return
		}

		// 将用户信息存入上下文
		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("claims", claims)

		c.Next()
	}
}

// OptionalJWTAuthMiddleware 可选JWT认证中间件（不强制要求登录）
func OptionalJWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := ParseToken(parts[1])
		if err == nil {
			c.Set("userId", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("claims", claims)
		}

		c.Next()
	}
}
