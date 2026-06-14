package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count     int
	resetTime time.Time
}

func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	visitors := map[string]*visitor{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		mu.Lock()
		v := visitors[ip]
		if v == nil || now.After(v.resetTime) {
			v = &visitor{resetTime: now.Add(window)}
			visitors[ip] = v
		}
		v.count++
		allowed := v.count <= limit
		mu.Unlock()

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{"code": http.StatusTooManyRequests, "message": "too many requests"})
			c.Abort()
			return
		}
		c.Next()
	}
}
