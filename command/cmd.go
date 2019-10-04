package command

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
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
		CmdString: cmdParts[0],
		LogFile:   fmt.Sprintf("./logs/%s", filePath),
	}
	return
}

// Start ...
func (c *Command) Start() {
	fmt.Println("  start cmd:", c.ID)
	cmd := exec.Command("bash", "-c", c.CmdString)

	// setup file
	c.LogFile = fmt.Sprintf("./logs/%s.out", c.ID)
	logFile, err := os.Create(c.LogFile)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	logFile.WriteString(fmt.Sprintf("Start:%s\n", c.StartTime.Format(time.RFC3339)))
	logFile.WriteString(fmt.Sprintf("Command:%s\n\n", c.CmdString))

	//
	// use bufio if i need to write to ws and file
	//

	// r, w := io.Pipe()
	// cmd.Stdout = w
	// s := bufio.NewScanner(r)
	// for s.Scan() {
	// 	bytes := s.Bytes()
	// 	outfile.Write(bytes)

	// 	// TODO: write to ws if needed?

	// 	// fmt.Println(string(s.Bytes()))
	// 	// if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
	// 	// 	fmt.Println("WriteMessage:", err)
	// 	// 	ws.Close()
	// 	// 	break
	// 	// }
	// }
	// w.Close()

	//
	//
	// just to file
	//

	// // open the out file for writing
	cmd.Stdout = logFile

	// start cmd
	err = cmd.Start()
	c.Running = true
	if err != nil {
		panic(err)
	}
	cmd.Wait()
}

// Close ...
func (c *Command) Close() {
	// TODO: rename file to "done"
	// write to file done time

	c.Cmd.Process.Kill()
}

// RunBash ...
func RunBash(commandString, dir string, env []string) (stdOut string, err error) {
	cmd := exec.Command("bash", "-c", commandString)
	if len(env) > 0 {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}
	var stdOutBuf bytes.Buffer
	var stdErrBuf bytes.Buffer
	cmd.Stdout = &stdOutBuf
	cmd.Stderr = &stdErrBuf
	cmd.Run()
	stdOut = strings.TrimSuffix(stdOutBuf.String(), "\n")
	stdErr := strings.TrimSuffix(stdErrBuf.String(), "\n")
	if stdErr != "" {
		err = errors.New(stdErr)
	}
	return
}

// ScanBash ...
func ScanBash(commandString, dir string, env []string) (cmd *exec.Cmd, outPipe, errPipe *bufio.Scanner) {
	cmd = exec.Command("bash", "-c", commandString)
	if len(env) > 0 {
		cmd.Env = env
	}
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, _ := cmd.StdoutPipe()
	outPipe = bufio.NewScanner(stdout)

	stderr, _ := cmd.StderrPipe()
	errPipe = bufio.NewScanner(stderr)

	cmd.Stdin = os.Stdin

	go cmd.Run()

	return
}
