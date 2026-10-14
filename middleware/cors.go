package middleware

import (
	"chatroombackend/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func AllowedOrigins() []string {
	out := []string{}
	if c := config.GetConfig(); c != nil {
		for _, v := range c.Server.AllowedOrigins {
			v = strings.TrimSpace(v)
			if v != "" && v != "*" {
				out = append(out, v)
			}
		}
		if len(out) > 0 {
			return out
		}
		// Existing YAML files can omit this new field; allow their server origin
		// as well as the normal local frontend without requiring an env file.
		if u, e := url.Parse(c.Server.BaseURL); e == nil && u.Host != "" {
			out = append(out, u.Scheme+"://"+u.Host)
			if u.Port() != "" {
				host := u.Hostname()
				if strings.Contains(host, ":") {
					host = "[" + host + "]"
				}
				out = append(out, u.Scheme+"://"+host)
			}
		}
	}
	return append(out, "http://127.0.0.1:11451", "http://localhost:11451")
}
func IsAllowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, v := range AllowedOrigins() {
		if origin == v {
			return true
		}
	}
	return false
}
func CORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{AllowOrigins: AllowedOrigins(), AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}, AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"}, ExposeHeaders: []string{"Retry-After", "X-Request-ID"}, MaxAge: 12 * time.Hour})
}
