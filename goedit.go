package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sourcegraph/go-lsp"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var diagnostics *tview.TextView

// Request JSON-RPC request structure
type Request[T any] struct {
	JsonRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  T      `json:"params,omitempty"`
}

type Response[T any] struct {
	JsonRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  T      `json:"result,omitempty"`
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
		default:

		}
		return event // Pass the key event back to the editor
	})

	logf("starting pipe listeners")
	// Start listening to gopls' output
	go func() {
		defer logf("Stopped listening to gopls")
		listenToGopls(stdout, diagnostics)
		logf("gopls state %s", cmd.ProcessState.String())
	}()

	go func() {
		defer logf("Stopped listening err gopls")
		listenForErrors(errPipe, diagnostics)
	}()

	if resp, err := sendInitializationRequest(stdin); err != nil {
		logf("Error sending initialization request: %v", err)
		return
	} else {
		logf("Response received: %+v", resp)
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

func sendInitializationRequest(stdin io.Writer) (Response[lsp.InitializeResult], error) {
	p := lsp.InitializeParams{
		RootURI:               "",
		ClientInfo:            lsp.ClientInfo{},
		Trace:                 "",
		InitializationOptions: nil,
		Capabilities:          lsp.ClientCapabilities{},
		WorkDoneToken:         "",
	}
	rq := req[lsp.InitializeParams]("initialize", p)
	return sendSync[lsp.InitializeParams, lsp.InitializeResult](stdin, rq)
}

var gid = int32(0)

func req[REQ any](method string, p REQ) Request[REQ] {
	id := atomic.AddInt32(&gid, 1)
	return Request[REQ]{
		JsonRPC: "2.0",
		ID:      int(id),
		Method:  method,
		Params:  p,
	}
}

func sendAsync[REQ any](stdin io.Writer, r Request[REQ]) error {
	err := send(stdin, r)
	return err
}

func sendSync[REQ any, RESP any](stdin io.Writer, r Request[REQ]) (Response[RESP], error) {
	var rtn Response[RESP]
	ch := make(chan []byte)
	addOutstandingMethod(r.ID, ch)
	defer removeOutstandingMethod(r.ID)
	err := send(stdin, r)
	if err != nil {
		return rtn, err
	}
	//TOOD put a timeout here
	data := <-ch
	err = json.Unmarshal(data, &rtn)
	return rtn, err
}

func send[R any](stdin io.Writer, request Request[R]) error {
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

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	uri := lsp.DocumentURI("file://" + absPath)
	request := Request[lsp.TextDocumentItem]{
		JsonRPC: "2.0",
		ID:      2,
		Method:  "textDocument/didOpen",
		Params: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: "go",
			Version:    1,
			Text:       string(content),
		},
	}
	return sendAsync(stdinWriter, request)
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

func addOutstandingMethod(id int, ch chan []byte) {
	omlock.Lock()
	defer omlock.Unlock()
	outstandingMethods[id] = ch
}

func removeOutstandingMethod(id int) {
	omlock.Lock()
	defer omlock.Unlock()
	delete(outstandingMethods, id)
}

var omlock = sync.Mutex{}
var outstandingMethods = make(map[int]chan []byte)

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
				_, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength)
				if err != nil {
					panic(err)
				}
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

	type jsonRpcResponse struct {
		JsonRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
	}
	var resp = jsonRpcResponse{}
	err = json.Unmarshal(buffer, &resp)
	if err != nil {
		logf("Error parsing JSON-RPC response: %v", err)
		panic(err)
	}
	omlock.Lock()
	if listener, ok := outstandingMethods[resp.ID]; ok {
		listener <- buffer
	} else {
		logf("No listener for %d", resp.ID)
	}
	omlock.Unlock()

	//// Parse the JSON-RPC response
	//var response map[string]interface{}
	//err = json.Unmarshal(buffer, &response)
	//if err != nil {
	//	logf("Error parsing JSON-RPC response: %v", err)
	//	panic(err)
	//}
	//
	//logf("Response received: %+v", response)
	//
	//// Check if it's a diagnostic notification and handle accordingly
	//if response["method"] == "textDocument/publishDiagnostics" {
	//	params, ok := response["params"].(map[string]interface{})
	//	if !ok {
	//		logf("Invalid diagnostic params format")
	//		panic(err)
	//	}
	//
	//	// Display diagnostics in the diagnostics pane
	//	//diagnostics.Clear() // Clear the panel before adding new diagnostics
	//	diagnosticItems, ok := params["diagnostics"].([]interface{})
	//	if !ok {
	//		logf("Invalid diagnostics format")
	//		panic(err)
	//	}
	//
	//	for _, diag := range diagnosticItems {
	//		diagnostic := diag.(map[string]interface{})
	//		message := fmt.Sprintf("- %s (line %v)\n", diagnostic["message"], diagnostic["range"])
	//		diagnostics.Write([]byte(message)) // Write diagnostic message to the panel
	//	}
	//}
}

func sendCompletionRequest(stdin io.Writer, uri string, line, character int) (Response[lsp.CompletionList], error) {
	r := lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: lsp.DocumentURI(uri),
			},
			Position: lsp.Position{
				Line:      line,
				Character: character,
			},
		},
		Context: lsp.CompletionContext{
			TriggerKind:      lsp.CTKInvoked,
			TriggerCharacter: "",
		},
	}
	rq := req[lsp.CompletionParams]("textDocument/completion", r)
	return sendSync[lsp.CompletionParams, lsp.CompletionList](stdin, rq)
}

func sendFormattingRequest(stdin io.Writer, uri string) error {
	p := lsp.DocumentFormattingParams{
		TextDocument: lsp.TextDocumentIdentifier{
			URI: lsp.DocumentURI(uri),
		},
		Options: lsp.FormattingOptions{
			TabSize:      2,
			InsertSpaces: true,
		},
	}
	r := req[lsp.DocumentFormattingParams]("textDocument/formatting", p)
	return sendAsync(stdin, r)
}
