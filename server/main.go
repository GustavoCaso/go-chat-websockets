package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

var (
	port uint
	wg   sync.WaitGroup
)

func init() {
	flag.UintVar(&port, "port", 8080, "Port to start the chat server")
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Info("Shutting Down!!!")
		cancel()
	}()

	hub := newHub(ctx, &wg)
	serverAddr := fmt.Sprintf(":%d", port)
	httpServer := httpServer(serverAddr, router(hub))
	go gracefullShutdown(ctx, httpServer, &wg)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Fatalf("Could not listen on %s", serverAddr)
	}

	wg.Wait()
}

func gracefullShutdown(ctx context.Context, server *http.Server, wg *sync.WaitGroup) {
	wg.Add(1)
loop:
	for {
		select {
		case <-ctx.Done():
			log.Info("Server is shutting down...")

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			server.SetKeepAlivesEnabled(false)
			if err := server.Shutdown(ctx); err != nil {
				log.WithError(err).Warn("Could not gracefully shutdown the server")
			}
			log.Info("Server shut down!")
			break loop
		}
	}
	wg.Done()
}

func router(hub *hub) *httprouter.Router {
	router := httprouter.New()

	router.GET("/chat/:chat_room/:user_name", hub.chatRoom)

	return router
}

func httpServer(addr string, router *httprouter.Router) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
}
