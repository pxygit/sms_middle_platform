package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func adminID(c *gin.Context) uint {
	value, ok := c.Get("adminID")
	if !ok {
		return 0
	}
	id, ok := value.(uint)
	if !ok {
		return 0
	}
	return id
}

func limit(c *gin.Context) int {
	value, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if value <= 0 || value > 200 {
		return 50
	}
	return value
}

func offset(c *gin.Context) int {
	value, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if value < 0 {
		return 0
	}
	return value
}

func stringID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
