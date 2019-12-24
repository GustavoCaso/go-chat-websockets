package main

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

type chat struct {
	name         string
	users        []*user
	messages     chan message
	messagesRead []message
	addedUsers   chan *user
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
	c.addedUsers <- user
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
	log.WithFields(log.Fields{
		"chat": c.name,
	}).Info("Listeing for messages")
	for {
		if len(c.users) > 0 {
			for _, user := range c.users {
				if !user.listening {
					user.listening = true
					go c.listenToUser(user)
				}
			}
		}
	}
}

func (c *chat) listenToUser(user *user) {
	c.wg.Add(1)
	for {
		log.WithFields(log.Fields{
			"username": user.name,
		}).Info("Listeing to incomming messages")
		_, msg, err := user.conn.Read(c.ctx)
		if err == nil {
			log.WithFields(log.Fields{
				"username": user.name,
			}).Info("Message received")
			c.messages <- message{
				bytes:  msg,
				author: user,
			}
		} else {
			log.WithError(err).WithFields(log.Fields{
				"username": user.name,
			}).Warn("Error receiving message")
			c.dropUsers <- user
			break
		}
	}
	wg.Done()
}

func (c *chat) broadcast() {
	c.wg.Add(1)
	log.Info("Broadcasting messages")
loop:
	for {
		select {
		case message := <-c.messages:
			log.WithField("from", message.author.name).Info("Received Message")
			usersToSend := c.userToSend(message.author)
			log.WithField("to", usersToSend).Info("Broadcasting message")
			bytes, err := message.print()
			if err == nil {
				for _, user := range usersToSend {
					user.conn.Write(c.ctx, websocket.MessageText, bytes)
				}
				c.messagesRead = append(c.messagesRead, message)
			} else {
				log.WithError(err).Warn("Error building the message")
			}
		case <-c.ctx.Done():
			break loop
		}
	}
	c.wg.Done()
}

func (c *chat) cleanup() {
	c.wg.Add(1)
	log.Infoln("Cleaning dropped users")
loop:
	for {
		select {
		case user := <-c.addedUsers:
			log.WithField("username", user.name).Info("Adding user")
			users := c.users
			users = append(users, user)
			c.users = users
			c.broadcastMessage([]byte(fmt.Sprintf("%s joined %s", user.name, c.name)))
		case user := <-c.dropUsers:
			log.WithField("username", user.name).Info("Removing user")
			users := c.deleteUser(user)
			c.users = users
		case <-c.ctx.Done():
			break loop
		}
	}
	c.wg.Done()
}

func (c *chat) broadcastMessage(message []byte) {
	for _, user := range c.users {
		user.conn.Write(c.ctx, websocket.MessageText, message)
	}
}
