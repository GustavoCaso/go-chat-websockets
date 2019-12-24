package main

import (
	"context"
	"sync"
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
