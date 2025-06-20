package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Axpz/xHub/internal/server"
	"github.com/Axpz/xHub/internal/store"
)

const (
	HTTPPort = 20080
	GRPCPort = 20081
)

func main() {
	log.Println("Starting xHub Server...")

	// 创建共享的数据存储
	store := store.NewAgentStore()

	// 创建HTTP服务器，使用共享的store
	httpServer := server.NewHTTPServer(store)

	// 创建gRPC服务器，使用共享的store
	grpcServer := server.NewGRPCServerWithStore(store)

	// 启动HTTP服务器
	go func() {
		log.Printf("Starting HTTP server on port %d", HTTPPort)
		if err := httpServer.Start(HTTPPort); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// 启动gRPC服务器
	go func() {
		log.Printf("Starting gRPC server on port %d", GRPCPort)
		if err := grpcServer.Start(GRPCPort); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Received shutdown signal, gracefully shutting down...")

	// 优雅关闭
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		grpcServer.Stop()
		log.Println("gRPC server stopped")
	}()

	wg.Wait()
	log.Println("Server shutdown complete")
}
