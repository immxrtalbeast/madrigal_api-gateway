package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(appSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if authHeader == "" {
			var err error
			tokenString, err = c.Cookie("jwt")
			if err != nil {
				c.AbortWithStatusJSON(401, gin.H{"error": "JWT required"})
				return
			}
		}

		if tokenString == authHeader {
			c.AbortWithStatusJSON(401, gin.H{"error": "Bearer token required"})
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(appSecret), nil
		})

		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token: " + tokenString})
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token claims"})
			return
		}

		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				c.AbortWithStatusJSON(401, gin.H{"error": "Token expired"})
				return
			}
		}

		userID, ok := claims["uid"]
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid user ID in token"})
			return
		}

		c.Set("userID", userID)

		c.Next()
	}
}
