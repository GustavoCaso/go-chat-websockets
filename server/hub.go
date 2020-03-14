package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type hub struct {
	rooms map[string]*chat
	ctx   context.Context
	wg    *sync.WaitGroup
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
			}
		}
	}
}

func (h *hub) addChat(chatName string) *chat {
	newChat := &chat{
		name:       chatName,
		users:      []*user{},
		messages:   make(chan message, 100),
		dropUsers:  make(chan *user, 100),
		addedUsers: make(chan *user, 100),
		ctx:        h.ctx,
		wg:         h.wg,
	}
	h.rooms[chatName] = newChat
	return newChat
}
