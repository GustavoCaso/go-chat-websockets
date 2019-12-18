package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
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
		fmt.Println("Shutting Down!!!")
		cancel()
	}()

	logger := log.New(os.Stdout, "[HTTP] ", log.LstdFlags)
	hub := newHub(ctx, &wg)
	serverAddr := fmt.Sprintf(":%d", port)
	httpServer := httpServer(serverAddr, router(hub), logger)
	go gracefullShutdown(ctx, httpServer)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", serverAddr, err)
	}
}

func gracefullShutdown(ctx context.Context, server *http.Server) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Server is shutting down...")

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			server.SetKeepAlivesEnabled(false)
			if err := server.Shutdown(ctx); err != nil {
				fmt.Println("Could not gracefully shutdown the server")
			}
			fmt.Println("Server shut down!")
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
			fmt.Println("Error adding user to chat")
			panic("Error creating the user")
		}
		c.addUser(user)
		c.broadcastMessage([]byte(fmt.Sprintf("%s joined", userName)))
		c.run()
	} else {
		if c.hasUser(userName) {
			fmt.Println("Chat: ", chatRoom, " and user name: ", userName, " already exists")
		} else {
			user, err := newUser(userName, w, r)
			if err != nil {
				fmt.Println("Error adding user to chat")
				panic("Error creating the user")
			} else {
				c.addUser(user)
				fmt.Println(userName, " joined")
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

func httpServer(addr string, router *httprouter.Router, logger *log.Logger) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      router,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
}
