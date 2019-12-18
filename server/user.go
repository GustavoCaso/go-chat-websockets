package main

import (
	"fmt"
	"net/http"

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
		fmt.Println(err)
		return &user{}, err
	}
	user := &user{
		name:      userName,
		conn:      conn,
		listening: false,
	}
	return user, nil
}
