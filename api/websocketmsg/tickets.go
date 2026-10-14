package websocketmsg

import (
	"chatroombackend/middleware"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

type ticket struct {
	Claims  *middleware.CustomClaims
	Expires time.Time
}
type ticketBucket struct {
	At    time.Time
	Count int
}
type TicketStore struct {
	mu     sync.Mutex
	values map[string]ticket
	limits map[string]ticketBucket
}

func NewTicketStore() *TicketStore {
	return &TicketStore{values: map[string]ticket{}, limits: map[string]ticketBucket{}}
}
func (s *TicketStore) Mint(c *gin.Context) {
	claims := c.MustGet("claims").(*middleware.CustomClaims)
	now := time.Now()
	expires := now.Add(time.Minute)
	if claims.ExpiresAt.Time.Before(expires) {
		expires = claims.ExpiresAt.Time
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		c.JSON(503, gin.H{"code": 503, "message": "服务不可用"})
		return
	}
	key := base64.RawURLEncoding.EncodeToString(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, t := range s.values {
		if !now.Before(t.Expires) {
			delete(s.values, k)
		}
	}
	for k, b := range s.limits {
		if now.Sub(b.At) >= time.Minute {
			delete(s.limits, k)
		}
	}
	b := s.limits[claims.UserID]
	if b.Count >= 60 || len(s.values) >= 10000 {
		c.Header("Retry-After", "60")
		c.JSON(429, gin.H{"code": 429, "businessCode": "RATE_LIMITED", "message": "连接过于频繁"})
		return
	}
	if b.Count == 0 {
		b.At = now
	}
	b.Count++
	s.limits[claims.UserID] = b
	s.values[key] = ticket{claims, expires}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"code": 200, "data": gin.H{"ticket": key, "expiresAt": expires}})
}
func (s *TicketStore) Consume(key string) (*middleware.CustomClaims, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.values[key]
	delete(s.values, key)
	if !ok || !time.Now().Before(t.Expires) || !time.Now().Before(t.Claims.ExpiresAt.Time) {
		return nil, errors.New("连接票据无效")
	}
	return t.Claims, nil
}
