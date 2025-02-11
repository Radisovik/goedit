package main

import (
	"log"
	"os/exec"
)

func main() {
	cmd := exec.Command("/opt/homebrew/bin/gopls")
	//cmd := exec.Command("/opt/homebrew/bin/gopls", "-rpc.trace", "-logfile", "gopls.log")
	output, err := cmd.CombinedOutput()
	log.Printf("gopls output: %s", output)
	log.Printf("Starting gopll error %v", err)
}
