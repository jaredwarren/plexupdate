package command

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ID string

	Cmd       string
	Timestamp string

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

	// timestamp, so if we runt the exact same command it doesn't conflict
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	bs := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s", commandString, timestamp)))

	c := &Client{
		ID:        string(bs),
		StdOut:    make(chan string, 256),
		StdErr:    make(chan string, 256),
		pwd:       pwd,
		Timestamp: timestamp,
		Cmd:       commandString,
		stop:      make(chan bool),
	}

	go c.Start()

	return c
}

// Close everying
func (c *Client) Close() {
	fmt.Println("Closing cmd.Client")
	// c.cmd.Process.Kill()
	close(c.StdErr)
	close(c.StdOut)
	close(c.stop)
}

// Start command and write to StdOut and StdErr
func (c *Client) Start() {
	fmt.Println("  start cmd:", c.ID)

	cmd := exec.Command("echo", "'WHAT THE HECK IS UP'")

	// open the out file for writing
	outfile, err := os.Create(fmt.Sprintf("./%s.out", c.ID))
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	cmd.Stdout = outfile

	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	cmd.Wait()
	return

	//
	//
	//
	//

	// cmd := exec.Command("bash", "-c", c.Cmd)

	// r, w := io.Pipe()
	// cmd.Stdout = w
	// // c1.Stdin = r

	// // var b2 bytes.Buffer

	// cmd.Start()
	// // go cmd.Wait()

	// // time.Sleep(5 * time.Second)

	// s := bufio.NewScanner(r)
	// for s.Scan() {
	// 	fmt.Println(string(s.Bytes()))
	// 	// if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
	// 	// 	fmt.Println("WriteMessage:", err)
	// 	// 	ws.Close()
	// 	// 	break
	// 	// }
	// }
	// w.Close()

	//
	//
	//

	// args := []string{"bash", "-c", c.Cmd}

	// var err error
	// if args[0], err = exec.LookPath(args[0]); err != nil {
	// 	panic(err.Error())
	// }

	// // stdOut
	// outr, outw, err := os.Pipe()
	// if err != nil {
	// 	panic(err.Error())
	// }
	// defer outr.Close()
	// defer outw.Close()

	// // stdIn
	// inr, inw, err := os.Pipe()
	// if err != nil {
	// 	panic(err.Error())
	// }
	// defer inr.Close()
	// defer inw.Close()

	// // Process
	// proc, err := os.StartProcess(args[0], args, &os.ProcAttr{
	// 	Files: []*os.File{inr, outw, outw},
	// })
	// if err != nil {
	// 	panic(err.Error())
	// }

	// inr.Close()
	// outw.Close()

	// s := bufio.NewScanner(outr)
	// for s.Scan() {
	// 	ws.SetWriteDeadline(time.Now().Add(writeWait))
	// 	if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
	// 		fmt.Println("WriteMessage:", err)
	// 		ws.Close()
	// 		break
	// 	}
	// }
	// if s.Err() != nil {
	// 	fmt.Println("scan:", s.Err())
	// }

	//
	//
	//
	//

	// stdout, err := c.cmd.StdoutPipe()
	// if err != nil {
	// 	panic(err)
	// }
	// stderr, err := c.cmd.StderrPipe()
	// if err != nil {
	// 	panic(err)
	// }
	// c.cmd.Start()

	// // scan stderr
	// go func() {
	// 	scanner := bufio.NewScanner(stderr)
	// 	scanner.Split(bufio.ScanLines)
	// 	for scanner.Scan() {
	// 		c.StdErr <- scanner.Text()
	// 	}
	// }()

	// // scan stdout
	// scanner := bufio.NewScanner(stdout)
	// scanner.Split(bufio.ScanLines)
	// for scanner.Scan() {
	// 	c.StdOut <- scanner.Text()
	// }

	// c.cmd.Wait()
	fmt.Println("  cmd Done!", c.ID)
}

// Broadcast used to receive message from ws, or other client/hub
func (c *Client) Broadcast(msg []byte) {
	fmt.Printf("---> Message::::::%s\n", msg)
}

// Broadcast used to receive message from ws, or other client/hub
func (c *Client) Write(p []byte) (n int, err error) {
	fmt.Printf("---> TODO: something???::::::%s\n", p)
	return len(p), nil
}
