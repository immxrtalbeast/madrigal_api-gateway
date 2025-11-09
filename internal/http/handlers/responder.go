package handlers

import "github.com/gin-gonic/gin"

func writeJSON(c *gin.Context, status int, payload interface{}) {
	if payload == nil {
		c.Status(status)
		return
	}
	c.JSON(status, payload)
}

func writeError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
