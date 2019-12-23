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

type hub struct {
	rooms map[string]*chat
	ctx   context.Context
	wg    *sync.WaitGroup
}

var (
	port uint
	wg   sync.WaitGroup
)

func init() {
	flag.UintVar(&port, "port", 8080, "Port to start the health server")
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
	go gracefullShutdown(ctx, httpServer)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Fatalf("Could not listen on %s", serverAddr)
	}
}

func gracefullShutdown(ctx context.Context, server *http.Server) {
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
			break
		}
	}
}

func router(hub *hub) *httprouter.Router {
	router := httprouter.New()

	router.GET("/chat/:user_name/:chat_room", hub.chatRoom)

	return router
}

func newHub(ctx context.Context, wg *sync.WaitGroup) *hub {
	return &hub{
		rooms: make(map[string]*chat),
		ctx:   ctx,
		wg:    wg,
	}
}

func (h *hub) chatRoom(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatRoom := ps.ByName("chat_room")
	userName := ps.ByName("user_name")
	c, ok := h.rooms[chatRoom]
	if !ok {
		c := h.addChat(chatRoom)
		user, err := newUser(userName, w, r)
		if err != nil {
			log.WithError(err).Fatal("Error creating user to new chat")
		}
		c.addUser(user)
		c.broadcastMessage([]byte(fmt.Sprintf("%s joined", userName)))
		c.run()
	} else {
		if c.hasUser(userName) {
			log.WithFields(log.Fields{
				"chat":     chatRoom,
				"username": userName,
			}).Info("User already exists in chat room")
		} else {
			user, err := newUser(userName, w, r)
			if err != nil {
				log.WithError(err).Fatal("Error creating user for chat")
			} else {
				c.addUser(user)
				log.WithFields(log.Fields{
					"chat":     chatRoom,
					"username": userName,
				}).Info("User joined")
				c.broadcastMessage([]byte(fmt.Sprintf("%s joined", userName)))
			}
		}
	}
}

func (h *hub) addChat(chatName string) *chat {
	newChat := &chat{
		name:      chatName,
		users:     []*user{},
		messages:  make(chan message, 100),
		dropUsers: make(chan *user, 100),
		ctx:       h.ctx,
		wg:        h.wg,
	}
	h.rooms[chatName] = newChat
	return newChat
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
