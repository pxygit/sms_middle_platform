package middleware

import (
	"strings"

	"sms-middle-platform/backend/internal/auth"
	"sms-middle-platform/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func AdminAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenValue := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenValue == "" || tokenValue == header {
			response.Unauthorized(c, "missing authorization token")
			c.Abort()
			return
		}
		claims, err := auth.ParseToken(secret, tokenValue)
		if err != nil {
			response.Unauthorized(c, "invalid authorization token")
			c.Abort()
			return
		}
		c.Set("adminID", claims.AdminID)
		c.Set("adminUsername", claims.Username)
		c.Set("adminRole", claims.Role)
		c.Next()
	}
}
