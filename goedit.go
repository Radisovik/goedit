package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	cmd := exec.Command("/opt/homebrew/bin/gopls", "-vv", "-rpc.trace", "-logfile", "gopls.log")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logf("Error creating stdin pipe: %v", err)
		return
	}
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

	if err := sendInitializationRequest(stdin); err != nil {
		logf("Error sending initialization request: %v", err)
		return
	}

	if err := sendDidOpen(stdin, "testdata/testprogram.go"); err != nil {
		logf("Error sending initialization request: %v", err)
		return
	}

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

	return send(stdin, initRequest)
}

func send(stdin io.Writer, request Request) error {
	// Marshal the request into JSON
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	fullRequest := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)

	_, err = stdin.Write([]byte(fullRequest))
	if err != nil {
		logf("Error writing to stdin: %v", err)
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

	//var joe TextDocumentItem

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	uri := "file://" + absPath
	request := Request{
		JsonRPC: "2.0",
		ID:      2,
		Method:  "textDocument/didOpen",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        uri,
				"languageId": "go",
				"version":    1,
				"text":       string(content),
			},
		},
	}
	return send(stdinWriter, request)
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

	var contentLength int
	var lineBuffer bytes.Buffer
	for {
		// Read byte by byte to construct each line
		b := make([]byte, 1)
		_, err := stdout.Read(b)
		if err != nil {
			logf("Error reading from stdout: %v", err)
			return
		}
		if b[0] == '\n' {
			line := lineBuffer.String()
			lineBuffer.Reset()
			if line == "" { // Break on the empty line separating headers from the body
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				break
			}
		} else {
			lineBuffer.WriteByte(b[0])
		}
	}

	// If Content-Length is not found or reading fails, log the error
	if contentLength == 0 {
		logf("Invalid Content-Length or error reading headers")
		return
	}

	contentLength += 2 // Add 2 bytes for the newline characters and the JSON-RPC response
	// Allocate a buffer to read the response body of the specified size
	buffer := make([]byte, contentLength)
	n, err := io.ReadFull(stdout, buffer)
	if err != nil {
		logf("Error reading response body: %v", err)
		return
	}
	if n != contentLength {
		logf("Invalid response body size: expected %d, got %d", contentLength, n)
		panic("Invalid response body size")
	}

	// Parse the JSON-RPC response
	var response map[string]interface{}
	err = json.Unmarshal(buffer, &response)
	if err != nil {
		logf("Error parsing JSON-RPC response: %v", err)
		panic(err)
	}

	logf("Response received: %+v", response)

	// Check if it's a diagnostic notification and handle accordingly
	if response["method"] == "textDocument/publishDiagnostics" {
		params, ok := response["params"].(map[string]interface{})
		if !ok {
			logf("Invalid diagnostic params format")
			panic(err)
		}

		// Display diagnostics in the diagnostics pane
		//diagnostics.Clear() // Clear the panel before adding new diagnostics
		diagnosticItems, ok := params["diagnostics"].([]interface{})
		if !ok {
			logf("Invalid diagnostics format")
			panic(err)
		}

		for _, diag := range diagnosticItems {
			diagnostic := diag.(map[string]interface{})
			message := fmt.Sprintf("- %s (line %v)\n", diagnostic["message"], diagnostic["range"])
			diagnostics.Write([]byte(message)) // Write diagnostic message to the panel
		}
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
