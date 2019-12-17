package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"
	"nhooyr.io/websocket"
)

type Message struct {
	bytes  []byte
	author User
}

func (m Message) print() ([]byte, error) {
	buffer := bytes.NewBufferString(m.author.name + ": ")
	nWrite, err := buffer.Write(m.bytes)
	if err != nil {
		return []byte{}, err
	}
	if nWrite != len(m.bytes) {
		return []byte{}, errors.New("Error creating the message")
	}
	return buffer.Bytes(), nil
}

type User struct {
	name string
	conn *websocket.Conn
}

type Chat struct {
	name         string
	users        []User
	messages     chan Message
	messagesRead []Message
}

func (c Chat) hasUser(userName string) bool {
	for _, user := range c.users {
		if user.name == userName {
			return true
		}
	}
	return false
}

func (c Chat) addUser(userName string, w http.ResponseWriter, r *http.Request) error {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	user := User{
		name: userName,
		conn: conn,
	}
	users := c.users
	users = append(users, user)
	c.users = users
	return nil
}

func (c Chat) userToSend(author User) []User {
	result := []User{}
	for _, user := range c.users {
		if user != author {
			result = append(result, user)
		}
	}
	return result
}

func (c Chat) listen() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		for _, user := range c.users {
			_, msg, err := user.conn.Read(ctx)
			if err != nil {
				c.messages <- Message{
					bytes:  msg,
					author: user,
				}
			}
		}
	}
}

func (c Chat) broadcast() {
	for {
		select {
		case message := <-c.messages:
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			usersToSend := c.userToSend(message.author)
			bytes, err := message.print()
			if err != nil {
				fmt.Println(err)
				c.messagesRead = append(c.messagesRead, message)
				continue
			}
			for _, user := range usersToSend {
				user.conn.Write(ctx, websocket.MessageText, bytes)
			}
		}
	}
}

func (c Chat) broadcastMessage(message []byte) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, user := range c.users {
		user.conn.Write(ctx, websocket.MessageText, message)
	}
}

type hub struct {
	rooms                  map[string]Chat
	errorConnectionChannel chan *websocket.Conn
}

var (
	port uint
)

func init() {
	flag.UintVar(&port, "port", 8080, "Port to start the health server")
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "[HTTP] ", log.LstdFlags)
	hub := newHub()
	serverAddr := fmt.Sprintf(":%d", port)
	httpServer := httpServer(serverAddr, router(hub), logger)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", serverAddr, err)
	}
}

func router(hub *hub) *httprouter.Router {
	router := httprouter.New()

	router.GET("/chat/:user_name/:chat_room", hub.chatRoom)

	return router
}

func newHub() *hub {
	return &hub{
		rooms:                  make(map[string]Chat),
		errorConnectionChannel: make(chan *websocket.Conn),
	}
}

func (h *hub) chatRoom(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatRoom := ps.ByName("chat_room")
	userName := ps.ByName("user_name")
	c, ok := h.rooms[chatRoom]
	if !ok {
		c := h.addChat(chatRoom)
		err := c.addUser(userName, w, r)
		if err != nil {
			fmt.Println("Error adding user to chat")
		}
		fmt.Println("Broadcasting message to: ", c)
		c.broadcastMessage([]byte(fmt.Sprintf("%s joined", userName)))
		go c.listen()
		go c.broadcast()
	} else {
		if c.hasUser(userName) {
			fmt.Println("Chat: ", chatRoom, " and user name: ", userName, " already exists")
		} else {
			err := c.addUser(userName, w, r)
			if err != nil {
				fmt.Println("Error adding user to chat")
			} else {
				c.broadcastMessage([]byte(fmt.Sprintf("%s joined", userName)))
			}
		}
	}

}

func (h *hub) addChat(chat string) Chat {
	newChat := Chat{
		name:  chat,
		users: []User{},
	}
	h.rooms[chat] = newChat
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
