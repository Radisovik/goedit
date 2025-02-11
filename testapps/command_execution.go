package main

import (
	"bufio"
	"log"
	"os/exec"
)

func main() {
	// Set up the command
	cmd := exec.Command("/opt/homebrew/bin/gopls", "-vv", "-rpc.trace", "-logfile", "gopls.log")

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to get stderr: %v", err)
	}

	stdin, err := cmd.StdinPipe()

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start gopls: %v", err)
	}

	_, err = stdin.Write([]byte("hello\n"))
	if err != nil {
		log.Fatalf("Failed to write to stdin: %v", err)
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Printf("[stdout] %s", scanner.Text())
		}
	}()
	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[stderr] %s", scanner.Text())
		}
	}()

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		log.Printf("gopls exited with error: %v", err)
	} else {
		log.Printf("gopls exited successfully")
	}

}
