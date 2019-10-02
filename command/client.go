package command

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ID string

	cmd *exec.Cmd

	// Buffered channel of outbound messages.
	StdOut chan string
	StdErr chan string

	stop chan bool

	pwd string
}

// NewClient ...
func NewClient(commandString string) *Client {
	// TODO: optional pwd, env, ...
	pwd, _ := os.Getwd()

	// use base64 of command as ID, might be faster to get random, or use time.
	// Another idea, use mutex to ensure get_timestamp is unique
	bs := base64.StdEncoding.EncodeToString([]byte(commandString))

	c := &Client{
		ID:     string(bs),
		StdOut: make(chan string, 256),
		StdErr: make(chan string, 256),
		pwd:    pwd,
		cmd:    exec.Command("bash", "-c", commandString),
		stop:   make(chan bool),
	}

	go c.Start()

	return c
}

// Close everying
func (c *Client) Close() {
	fmt.Println("Closing cmd.Client")
	c.cmd.Process.Kill()
	close(c.StdErr)
	close(c.StdOut)
	close(c.stop)
}

// Start command and write to StdOut and StdErr
func (c *Client) Start() {
	fmt.Println("  start cmd:", c.ID)

	stdout, _ := c.cmd.StdoutPipe()
	stderr, _ := c.cmd.StderrPipe()
	c.cmd.Start()

	// scan stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			c.StdErr <- scanner.Text()
		}
	}()

	// scan stdout
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		c.StdOut <- scanner.Text()
	}

	c.cmd.Wait()
	fmt.Println("  cmd Done!", c.ID)
}

// Broadcast used to receive message from ws, or other client/hub
func (c *Client) Broadcast(msg []byte) {
	fmt.Printf("---> Message::::::%s\n", msg)
}
