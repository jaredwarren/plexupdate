package command

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Command ...
type Command struct {
	ID        string
	Running   bool
	LogFile   string
	CmdString string
	Cmd       *exec.Cmd
	Pwd       string
	StartTime time.Time

	logHandler *os.File
}

// NewCommand return new command
func NewCommand(cmd string) *Command {
	// TODO: optional pwd, env, ...
	pwd, _ := os.Getwd()

	// Generate ID: timestamp, so if we runt the exact same command it doesn't conflict
	now := time.Now()
	timestamp := strconv.FormatInt(now.UTC().UnixNano(), 10)
	bs := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%s", cmd, timestamp)))
	id := string(bs)

	c := &Command{
		ID:        id,
		LogFile:   "",
		Running:   false,
		Pwd:       pwd,
		CmdString: cmd,
		StartTime: now,
	}

	return c
}

// LoadFile create a command from log file
func LoadFile(filePath string) (cmd *Command) {
	parts := strings.Split(filePath, ".")
	ID := parts[0]

	dat, _ := base64.StdEncoding.DecodeString(ID)
	cmdParts := strings.Split(string(dat), "|")

	cmd = &Command{
		ID:        ID,
		Running:   false, // assume not running
		CmdString: cmdParts[0],
		LogFile:   fmt.Sprintf("./logs/%s", filePath),
	}
	return
}

// Start ...
func (c *Command) Start() {
	fmt.Println("  start cmd:", c.ID)
	c.Cmd = exec.Command("bash", "-c", c.CmdString)

	// setup file
	c.LogFile = fmt.Sprintf("./logs/%s.out", c.ID)
	logHandler, err := os.Create(c.LogFile)
	c.logHandler = logHandler
	if err != nil {
		panic(err)
	}
	defer func() {
		c.logHandler.Close()
		c.logHandler = nil
	}()

	c.logHandler.WriteString(fmt.Sprintf("Start:%s\n", c.StartTime.Format(time.RFC3339)))
	c.logHandler.WriteString(fmt.Sprintf("Command:%s\n\n", c.CmdString))

	// open the out file for writing
	c.Cmd.Stdout = c.logHandler
	c.Cmd.Stderr = c.logHandler // not sure if this is a good idea??

	// start cmd
	err = c.Cmd.Start()
	c.Running = true
	if err != nil {
		panic(err)
	}
	c.Cmd.Wait()
}

// Close ...
func (c *Command) Close() {
	// write to file done time, do first so file doesn't get closed
	if c.logHandler != nil {
		end := time.Now()
		c.logHandler.WriteString(fmt.Sprintf("\n\n\nEnd:%s\n\nΔ:%+v\n", end.Format(time.RFC3339), end.Sub(c.StartTime)))
	}

	if c.Cmd != nil {
		c.Cmd.Process.Kill()
	}
}
