package command

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/jaredwarren/plexupdate/hub"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ID  string
	Hub *hub.Hub

	cmd *exec.Cmd

	// Buffered channel of outbound messages.
	send chan []byte

	stop chan bool

	pwd string
}

// NewClient ...
func NewClient(command string) *Client {
	pwd, _ := os.Getwd()
	c := &Client{
		send: make(chan []byte, 256),
		pwd:  pwd,
	}

	go c.Start()

	return c
}

// Close ...
func (c *Client) Close() {
	fmt.Println("Closing cmd.Client")
	c.cmd.Process.Kill()
	close(c.send)
}

// Send ...
func (c *Client) Send(msg []byte) {

	// TODO: run command

	fmt.Println("TODO: run  command:", string(msg))
	c.send <- msg
}

// GetSend ...
func (c *Client) GetSend() chan []byte {
	return c.send
}

// GetName ...
func (c *Client) GetName() string {
	return "cmd"
}

// GetID ...
func (c *Client) GetID() string {
	return c.ID
}

// SetID ...
func (c *Client) SetID(id string) {
	c.ID = id
}

// Start ...
func (c *Client) Start() {
	for {
		select {
		case <-c.stop:
			c.Close()
			return
		case msgString := <-c.send:
			msg := hub.NewMessage(msgString)
			if msg.Sender == c.ID {
				continue
			}
			c.RunCmd(msg)
		}
	}
}

// TODO: creat option to run query

// TODO: add reminder tool...

// TODO: add upload file and/or download file

// RunCmd ...
func (c *Client) RunCmd(msg *hub.Message) {
	cmd := fmt.Sprintf("%v", msg.Data)
	// if cmd == "cd ...?" {
	// 	c.pwd = cmd
	// }
	// fmt.Println("TODO: run  command:", string(msg))
	out, err := RunBash(cmd, "", nil)
	if err != nil {
		c.Hub.Broadcast(&hub.Message{
			Sender: c.ID,
			Type:   "error",
			Action: cmd,
			Data:   err.Error(),
		})
	}

	fmt.Println(" # ", c, out, err)

	// TODO: cleanup logs
	//  - fix scrolling on cmd page!!!
	//  - output err too...
	//  - test vpn?
	//  - this whole thing might not be worth it...

	c.Hub.Broadcast(&hub.Message{
		Sender: c.ID,
		Type:   "message",
		Action: cmd,
		Data:   out,
	})
	// c.Hub.Broadcast([]byte(out))
}
