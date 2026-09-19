package internal

import (
	"net/http"
	"strings"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

func addMiddleware(g *gin.Engine) {
	g.Use(middlewareExample)
}

func middlewareExample(c *gin.Context) {
	// do something before request

	c.Next()
}

func addAuthMiddleware(b *backend) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Authorization format must be Bearer <token>",
			})
			c.Abort()
			return
		}

		if _, err := util.ValidateJWT(parts[1], b.jwt.secret); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Invalid token: " + err.Error(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
