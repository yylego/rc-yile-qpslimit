package service

import (
	"log"
	"math/rand/v2"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yylego/must"
	"github.com/yylego/rc-yile-qpslimit/internal/middleware"
)

func Run(maxQps int, address string, quit <-chan struct{}) {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.RateLimitMiddleware(maxQps, quit))

	engine.GET("/api/ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"value": rand.Int64()})
	})

	engine.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	log.Printf("[service] listening on %s (maxQps=%d)", address, maxQps)

	go func() {
		must.Done(http.ListenAndServe(address, engine))
	}()

	<-quit
	log.Println("[service] done")
}
