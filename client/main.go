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
	"time"

	log "github.com/sirupsen/logrus"
	"nhooyr.io/websocket"
)

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
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		log.WithError(err).Fatal("Error opening a connection to server")
	}
	log.Info("Connected")
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	go func() {
		for {
			_, reader, err := c.Reader(ctx)
			if err != nil {
				log.WithError(err).Warn("Error receiving message")
				break
			} else {
				io.Copy(os.Stdout, reader)
				fmt.Println("")
			}
		}
	}()

	input := make(chan string, 1)
	go getInput(input)

Loop:
	for {
		select {
		case text := <-input:
			err = c.Write(ctx, websocket.MessageText, []byte(text))
			if err != nil {
				log.WithError(err).Fatal("Error sending message")
				break
			}
		case <-ctx.Done():
			log.Fatal("Client session ended")
		case <-time.After(4000 * time.Millisecond):
			continue Loop
		}
	}
}

func getInput(input chan string) {
	for {
		in := bufio.NewReader(os.Stdin)
		result, err := in.ReadString('\n')
		if err != nil {
			log.WithError(err).Fatal(err)
		}

		input <- result
	}
}
