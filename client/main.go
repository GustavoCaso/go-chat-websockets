package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

type client struct {
	userName string
	conn     *websocket.Conn
	ctx      context.Context
	message  chan string
}

func (c *client) close() {
	defer c.conn.Close(websocket.StatusInternalError, "Client closed connection")
}

func (c *client) listen() {
	for {
		_, reader, err := c.conn.Reader(c.ctx)
		if err != nil {
			log.WithError(err).Warn("Error receiving message")
			break
		} else {
			io.Copy(os.Stdout, reader)
			fmt.Println("")
		}
	}
}

func (c *client) getInput() {
	for {
		in := bufio.NewReader(os.Stdin)
		result, err := in.ReadString('\n')
		if err != nil {
			log.WithError(err).Fatal(err)
		}

		c.message <- result
	}
}

func (c *client) run() {
	go c.listen()
	go c.getInput()
loop:
	for {
		select {
		case text := <-c.message:
			err := c.conn.Write(c.ctx, websocket.MessageText, []byte(text))
			if err != nil {
				log.WithError(err).Fatal("Error sending message")
				break
			}
		case <-c.ctx.Done():
			log.Info("Client session ended")
			break loop
		}
	}
	c.close()
}

func newClient(ctx context.Context, userName, addr string) *client {
	c, _, err := websocket.Dial(ctx, addr, nil)
	if err != nil {
		log.WithError(err).Fatal("Error opening a connection to server")
	}
	return &client{
		userName: userName,
		conn:     c,
		ctx:      ctx,
		message:  make(chan string, 1),
	}
}

var (
	port     uint
	chatRoom string
	user     string
)

func init() {
	flag.UintVar(&port, "port", 8080, "Port to start the health server")
	flag.StringVar(&chatRoom, "chat_room", "default", "Chat room to connect")
	flag.StringVar(&user, "user_name", "", "Username to connect as")
}

func main() {
	flag.Parse()

	if user == "" {
		log.Fatal("Please provide an username you want to log in")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Info("Shutting Down!!!")
		cancel()
	}()

	url := fmt.Sprintf("ws://localhost:%d/chat/%s/%s", port, user, chatRoom)
	client := newClient(ctx, user, url)

	client.run()
}
