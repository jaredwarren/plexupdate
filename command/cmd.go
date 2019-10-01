package command

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

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
