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
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("Shutting Down!!!")
		cancel()
	}()

	url := fmt.Sprintf("ws://localhost:%d/chat/%s/%s", port, user, chatRoom)
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Connected")
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	go func() {
		for {
			_, reader, err := c.Reader(ctx)
			if err != nil {
				fmt.Println("Error receiving message: ", err.Error())
				break
			} else {
				io.Copy(os.Stdout, reader)
				fmt.Println("")
			}
		}
	}()

	// send
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}

		err = c.Write(ctx, websocket.MessageText, []byte(text))
		if err != nil {
			fmt.Println("Error sending message: ", err.Error())
			break
		}
	}

}
