package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"io"
	"log"
	"os"
	"os/exec"
)

// JSON-RPC request structure
type Request struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

func main() {
	// Start the gopls process
	cmd := exec.Command("gopls")
	cmd.Dir = "./testdata"
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdin pipe: %v\n", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		return
	}

	// Start the gopls process
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting gopls: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "Error reading testprogram.go: %v\n", err)
		return
	}
	fileContent := string(fileContentBytes)

	editor.SetText(fileContent) // Set the initial content of the editor
	// Create a Flex Layout to organize the editor and diagnostics pane
	diagnostics := tview.NewTextView().
		SetText("[red]Diagnostics will appear here.[white]").
		SetDynamicColors(true)
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(editor, 0, 1, true).      // Editor takes most space
		AddItem(diagnostics, 3, 1, false) // Diagnostics pane is smaller

	// Handle key events (e.g., simulate saving or other commands)
	editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlS: // Ctrl+S saves the file
			//log.Println("File saved!") // Replace with actual save logic
		case tcell.KeyCtrlQ: // Exit app
			app.Stop()
		}
		return event // Pass the key event back to the editor
	})

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
		fmt.Fprintf(os.Stderr, "Error marshaling request: %v\n", err)
		return
	}

	// Send the request
	_, err = stdin.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to stdin: %v\n", err)
		return
	}
	stdin.Write([]byte("\n"))                           // JSON-RPC must end with a newline
	err = sendDidOpen(stdin, "testdata/testprogram.go") // Notify gopls that we opened a file
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending didOpen notification: %v\n", err)
		return
	}
	// Start listening to gopls' output
	go func() {
		listenToGopls(stdout, diagnostics)
	}()

	// Set up the application and run it
	if err := app.SetRoot(flex, true).Run(); err != nil {
		app.Stop()
		log.Fatalf("booom! %+v", err)
		//panic(err)
	}
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
func listenToGopls(stdout io.ReadCloser, diagnostics *tview.TextView) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var response map[string]interface{}
		err := json.Unmarshal(scanner.Bytes(), &response)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing gopls response: %v\n", err)
			continue
		}

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
