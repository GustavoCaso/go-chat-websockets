package main

import (
	"context"
	"fmt"
	"sync"

	"nhooyr.io/websocket"
)

type chat struct {
	name         string
	users        []*user
	messages     chan message
	messagesRead []message
	dropUsers    chan *user
	ctx          context.Context
	wg           *sync.WaitGroup
}

func (c *chat) hasUser(userName string) bool {
	for _, user := range c.users {
		if user.name == userName {
			return true
		}
	}
	return false
}

func (c *chat) addUser(user *user) {
	users := c.users
	users = append(users, user)
	c.users = users
}

func (c *chat) deleteUser(userToDelete *user) []*user {
	var result []*user
	for _, user := range c.users {
		if user != userToDelete {
			result = append(result, user)
		}
	}
	return result
}

func (c *chat) userToSend(author *user) []*user {
	result := []*user{}
	for _, user := range c.users {
		if user != author {
			result = append(result, user)
		}
	}
	return result
}

func (c *chat) run() {
	go c.listen()
	go c.broadcast()
	go c.cleanup()
}

func (c *chat) listen() {
	fmt.Println("Listeing for messages for chat ", c.name)
	for {
		for _, user := range c.users {
			if !user.listening {
				user.listening = true
				go c.listenToUser(user)
			}
		}
	}
}

func (c *chat) listenToUser(user *user) {
	for {
		fmt.Println("Listeing for messages for user ", user.name)
		msgType, msg, err := user.conn.Read(c.ctx)
		fmt.Println("Got message with type: ", msgType)
		if err == nil {
			fmt.Println("Message recieved: ", msg)
			c.messages <- message{
				bytes:  msg,
				author: user,
			}
		} else {
			fmt.Println("Error Message recieved: ", err)
			c.dropUsers <- user
			break
		}
	}
}

func (c *chat) broadcast() {
	fmt.Println("Broadcasting messages")
	for {
		select {
		case message := <-c.messages:
			fmt.Println("Broadcasting message: ", message)
			usersToSend := c.userToSend(message.author)
			fmt.Println("User to send message: ", usersToSend)
			bytes, err := message.print()
			fmt.Println("Message to send: ", bytes)
			if err == nil {
				for _, user := range usersToSend {
					fmt.Println("Broadcasting to: ", user.name)
					user.conn.Write(c.ctx, websocket.MessageText, bytes)
				}
				c.messagesRead = append(c.messagesRead, message)
			} else {
				fmt.Println("Error building the message: ", err)
			}
			fmt.Println("Finish broadcasting")
		}
	}
}

func (c *chat) cleanup() {
	fmt.Println("Cleaning dropped users")
	for {
		select {
		case user := <-c.dropUsers:
			fmt.Println("Removing user: ", user.name)
			users := c.deleteUser(user)
			c.users = users
		}
	}
}

func (c *chat) broadcastMessage(message []byte) {
	fmt.Println("Users in channel: ", len(c.users))
	for _, user := range c.users {
		user.conn.Write(c.ctx, websocket.MessageText, message)
	}
}
