package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yylego/ratelimit"
)

const RateLimitTokenName = "X-Rate-Limit-Token"

// RateLimitMiddleware creates a Gin middleware that limits QPS per key.
// Key is extracted from request header "X-Rate-Limit-Token".
// Requests without the key pass through, requests exceeding QPS get 429.
func RateLimitMiddleware(maxQps int) gin.HandlerFunc {
	group := ratelimit.NewGroup(maxQps, time.Second)
	return func(ctx *gin.Context) {
		key := ctx.GetHeader(RateLimitTokenName)
		if key == "" {
			ctx.Next()
			return
		}
		if !group.Allow(key) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "rate limited"})
			return
		}
		ctx.Next()
	}
}
