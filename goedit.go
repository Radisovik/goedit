package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var diagnostics *tview.TextView

// JSON-RPC request structure
type Request struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

var logfile *os.File
var logOpen sync.Once

func logf(format string, args ...interface{}) {
	logOpen.Do(func() {
		var err error
		logfile, err = os.OpenFile("goedit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			panic(err)
		}
	})
	format = strings.TrimSpace(format)

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err := logfile.WriteString(fmt.Sprintf("%s %s \n", timestamp, fmt.Sprintf(format, args...)))
	if err != nil {
		panic(err)
	}
}

func main() {

	logf("Starting goedit")
	// Start the gopls process
	cmd := exec.Command("/opt/homebrew/bin/gopls", "-rpc.trace", "-logfile", "gopls.log")

	//output, err2 := cmd.CombinedOutput()
	//if err2 != nil {
	//	logf("Error starting gopls: %v", err2)
	//	return
	//}
	//logf("gopls output: %s", output)
	//time.Sleep(20 * time.Second)
	//stdin, err := cmd.StdinPipe()
	//if err != nil {
	//	logf("Error creating stdin pipe: %v", err)
	//	return
	//}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logf("Error creating stdout pipe: %v", err)
		return
	}

	errPipe, err := cmd.StderrPipe()
	if err != nil {
		logf("Error creating stderr pipe: %v", err)
		return
	}

	// Start the gopls process
	logf("Starting gopls")
	if err := cmd.Start(); err != nil {
		logf("Error starting gopls: %v", err)
		return
	}

	// Create new application
	app := tview.NewApplication()

	// Create a TextView for displaying file content
	editor := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	// load testdata/testprogram.go into a string
	fileContentBytes, err := os.ReadFile("testdata/testprogram.go")
	if err != nil {
		logf("Error reading testprogram.go: %v", err)
		return
	}
	fileContent := string(fileContentBytes)

	editor.SetText(fileContent) // Set the initial content of the editor
	// Create a Flex Layout to organize the editor and diagnostics pane
	editor.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		//fmt.Fprintf(os.Stderr, "Mouse action: %v, Mouse event: %v\n", action, event)
		return action, event
	})

	diagnostics = tview.NewTextView().
		SetText("[red]Diagnostics will appear here.[white]").
		SetDynamicColors(true)
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(editor, 0, 1, true).      // Editor takes most space
		AddItem(diagnostics, 3, 1, false) // Diagnostics pane is smaller

	// Handle key events (e.g., simulate saving or other commands)
	editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		//editor.SetText(fmt.Sprintf("Key pressed: %q (ASCII: %d) %+v", event.Rune(), event.Key(), event))

		switch event.Key() {
		case tcell.KeyCtrlS: // Ctrl+S saves the file
			//log.Println("File saved!") // Replace with actual save logic
		case tcell.KeyCtrlQ: // Exit app
			app.Stop()
		}
		return event // Pass the key event back to the editor
	})

	logf("starting pipe listeners")
	// Start listening to gopls' output
	go func() {
		defer logf("Stopped listening to gopls")
		listenToGopls(stdout, diagnostics)
		cmd.Wait()
		logf("gopls state %s", cmd.ProcessState.String())
	}()

	go func() {
		defer logf("Stopped listening err gopls")
		listenForErrors(errPipe, diagnostics)
	}()

	//if err := sendInitializationRequest(stdin); err != nil {
	//	logf("Error sending initialization request: %v", err)
	//	return
	//}

	// Set up the application and run it
	if err := app.SetRoot(flex, true).Run(); err != nil {
		app.Stop()
		logf("booom! %+v", err)
		panic(err)
	}
}

func sendInitializationRequest(stdin io.Writer) error {
	// Example: Send Initialization Request
	initRequest := Request{
		JsonRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"processId": nil,
			"rootUri":   "file://" + os.Getenv("PWD"),
			"capabilities": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"completion": map[string]bool{
						"dynamicRegistration": false,
					},
				},
			},
		},
	}

	// Marshal the request into JSON
	data, err := json.Marshal(initRequest)
	if err != nil {
		logf("Error marshaling request: %v", err)
		return err
	}

	// Send the request
	_, err = stdin.Write(data)
	if err != nil {
		logf("Error writing to stdin: %v", err)
		return err
	}
	stdin.Write([]byte("\n")) // JSON-RPC must end with a newline
	logf("Sent initialization request")

	err = sendDidOpen(stdin, "testdata/testprogram.go") // Notify gopls that we opened a file
	if err != nil {
		logf("Error sending didOpen notification: %v", err)
		return err
	}
	return nil
}

// Open a file and notify gopls about it
func sendDidOpen(stdinWriter io.Writer, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	request := Request{
		JsonRPC: "2.0",
		ID:      2,
		Method:  "textDocument/didOpen",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file://" + filePath,
				"languageId": "go",
				"version":    1,
				"text":       string(content),
			},
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to stdin: %v", err)
	}

	_, err = stdinWriter.Write([]byte("\n")) // JSON-RPC messages end with a newline
	return err
}
func listenForErrors(errPipe io.ReadCloser, diagnostics *tview.TextView) {
	scanner := bufio.NewScanner(errPipe)
	for scanner.Scan() {
		logf("scanned error %s", scanner.Text())
	}
	if scanner.Err() != nil {
		logf("Error reading from gopls: %v", scanner.Err())
	} else {
		logf("done scanning gopls")
	}
}

func listenToGopls(stdout io.ReadCloser, diagnostics *tview.TextView) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		logf("scanned something %s", scanner.Text())

		var response map[string]interface{}
		err := json.Unmarshal(scanner.Bytes(), &response)
		if err != nil {
			logf("Error parsing gopls response: %v", err)
			continue
		}
		diagnostics.SetText(fmt.Sprintf("%v", response))
		// Check if it's a diagnostic notification
		if response["method"] == "textDocument/publishDiagnostics" {
			params := response["params"].(map[string]interface{})

			// Display diagnostics in the diagnostics pane
			diagnostics.Clear() // Clear the panel before adding new diagnostics
			for _, diag := range params["diagnostics"].([]interface{}) {
				diagnostic := diag.(map[string]interface{})
				message := fmt.Sprintf("- %s (line %v)\n", diagnostic["message"], diagnostic["range"])
				diagnostics.Write([]byte(message)) // Add each diagnostic message to the panel
			}
		}
	}
	if scanner.Err() != nil {
		logf("Error reading from gopls: %v", scanner.Err())
	} else {
		logf("done scanning gopls")
	}
}

func sendCompletionRequest(stdinWriter *os.File, uri string, line, character int) error {
	request := Request{
		JsonRPC: "2.0",
		ID:      3,
		Method:  "textDocument/completion",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]int{
				"line":      line,
				"character": character,
			},
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to stdin: %v", err)
	}
	_, err = stdinWriter.Write([]byte("\n"))
	return err
}

func sendFormattingRequest(stdinWriter *os.File, uri string) error {
	request := Request{
		JsonRPC: "2.0",
		ID:      4,
		Method:  "textDocument/formatting",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"options": map[string]interface{}{
				"tabSize":      2,
				"insertSpaces": true,
			},
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to stdin: %v", err)
	}

	_, err = stdinWriter.Write([]byte("\n"))
	return err
}

func sendDefinitionRequest(stdinWriter *os.File, uri string, line, character int) error {
	request := Request{
		JsonRPC: "2.0",
		ID:      5,
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]int{
				"line":      line,
				"character": character,
			},
		},
	}

	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	_, err = stdinWriter.Write(data)
	if err != nil {
		return fmt.Errorf("error writing to stdin: %v", err)
	}

	_, err = stdinWriter.Write([]byte("\n"))
	return err
}
