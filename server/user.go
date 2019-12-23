package main

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

type user struct {
	name      string
	conn      *websocket.Conn
	listening bool
}

func newUser(userName string, w http.ResponseWriter, r *http.Request) (*user, error) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.WithError(err).WithField("username", userName).Warn("Error open connection for user")
		return &user{}, err
	}
	user := &user{
		name:      userName,
		conn:      conn,
		listening: false,
	}
	return user, nil
}
