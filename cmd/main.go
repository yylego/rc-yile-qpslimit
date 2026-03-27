package main

import (
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/yylego/rc-yile-qpslimit/internal/service"
)

func main() {
	port := flag.Int("port", 8080, "HTTP port")
	maxQps := flag.Int("qps", 1000, "max QPS per token")
	flag.Parse()

	quit := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(quit)
	}()

	service.Run(*maxQps, ":"+strconv.Itoa(*port), quit)
}
