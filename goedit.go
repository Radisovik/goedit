package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Radisovik/goedit/editors"
	"github.com/gdamore/tcell/v2"
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

var logfile *os.File
var logOpen sync.Once

const NUM_LOG_LINES = 5
const MENU_LINE = 0
const FILE_TABS_LINE = 1
const EDITOR_LINE = 2

const ColorFaintGrey = tcell.ColorIsRGB | tcell.ColorValid | 0x323232

var LINE_NUMBERS_STYLE = tcell.Style{}.Foreground(tcell.ColorDarkGray)
var CODE_DEFAULT_STYLE = tcell.Style{}.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
var LOG_DEFAULT_STYLE = tcell.Style{}.Foreground(tcell.ColorLightSteelBlue).Background(tcell.ColorBlack)
var MENU_ENABLED_STYLE = tcell.Style{}.Foreground(tcell.ColorWhite).Background(ColorFaintGrey)
var MENU_DISABLED_STYLE = tcell.Style{}.Foreground(tcell.ColorDarkGray).Background(tcell.ColorBlack)
var FILE_TAB_STYLE = tcell.Style{}.Foreground(tcell.ColorYellow).Background(tcell.ColorBlack)
var currentFile = ""
var menuState = "disabled"
var LINE_NUMBERS_WIDTH = 4

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

var screen tcell.Screen
var logLines = [NUM_LOG_LINES]string{}

var files = make(map[string]editors.Editor)

type ViewArea struct {
	// absolute coordinates
	x, y, w, h     int
	scrollable     bool
	multiline      bool
	editable       bool
	content        editors.Editor
	dirty          bool
	topVisibleLine int
	focus          bool
}

func (va *ViewArea) render() {
	if va != nil && va.dirty && va.content != nil {
		x := va.x
		y := va.y
		for ln := 0; ln < va.content.Length(); ln++ {
			line, styles := va.content.GetLine(ln)
			for pos, r := range line {
				screen.SetContent(x, y, r, nil, styles[pos])
				x++
			}
			y++
		}
	}
}

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

	// Scroll the log lines array and add the new log line at the top
	for i := len(logLines) - 1; i > 0; i-- {
		logLines[i] = logLines[i-1]
	}
	msg := fmt.Sprintf("%s", fmt.Sprintf(format, args...))

	logLines[0] = msg

	if screen != nil {

		width, height := screen.Size()
		// Draw the log lines at the bottom of the screen
		for i, line := range logLines {
			drawText(0, height-len(logLines)+i, LOG_DEFAULT_STYLE, line)
			if len(line) < width {
				drawText(len(line), height-len(logLines)+i, LOG_DEFAULT_STYLE, strings.Repeat(" ", width-len(line)))
			}
		}
		screen.Show()
	}
}

var cx = 0
var cy = 0

var logArea *ViewArea
var editorArea *ViewArea
var menuArea *ViewArea
var tabsArea *ViewArea

func render() {
	logArea.render()
	editorArea.render()
	menuArea.render()
	tabsArea.render()
	screen.Show()
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

	logf("starting pipe listeners")
	// Start listening to gopls' output
	go func() {
		defer logf("Stopped listening to gopls")
		listenToGopls(stdout)
	}()

	go func() {
		defer logf("Stopped listening err gopls")
		listenForErrors(errPipe)
	}()

	screen, err = tcell.NewScreen()
	if err != nil {
		logf("Error creating screen: %v", err)
		return
	}
	if err := screen.Init(); err != nil {
		logf("Error initializing screen: %v", err)
		return
	}

	// Set default text style
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	screen.SetStyle(defStyle)

	screen.SetCursorStyle(tcell.CursorStyleBlinkingBar)
	screen.ShowCursor(cx+LINE_NUMBERS_WIDTH, cy+EDITOR_LINE)
	// Clear screen
	screen.Clear()

	quit := func() {
		// You have to catch panics in a defer, clean up, and
		// re-raise them - otherwise your application can
		// die without leaving any diagnostic trace.
		maybePanic := recover()
		screen.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}
	defer quit()

	// Event loop
	loadFiles()
	inited := false
	for {
		// update the tcell buffer from our text documents
		// and tell tcell to redraw
		render()

		// Poll event
		ev := screen.PollEvent()

		// Process event
		switch ev := ev.(type) {
		case *tcell.EventResize:
			if !inited {
				setupAreas()
				drawFileTabs()
				inited = true
			}
			screen.Sync()
		case *tcell.EventKey:
			if menuState == "enabled" {
				if ev.Rune() == 'Q' || ev.Rune() == 'q' {
					screen.Clear()
					return
				} else if ev.Rune() == 'L' || ev.Rune() == 'l' {
					screen.Sync()
				} else if ev.Rune() == 'T' || ev.Rune() == 't' {

				} else if ev.Rune() == 'R' || ev.Rune() == 'r' {

				} else if ev.Rune() == 'S' || ev.Rune() == 's' {

				} else {
					enableMenu(false)
				}

			} else {
				if ev.Key() == tcell.KeyEscape {
					enableMenu(true)

				} else if ev.Key() == tcell.KeyCtrlC {
					screen.Clear()
					return
				} else if ev.Key() == tcell.KeyDown {
					moveCursor(0, 1)
				} else if ev.Key() == tcell.KeyUp {
					moveCursor(0, -1)
				} else if ev.Key() == tcell.KeyLeft {
					moveCursor(-1, 0)
				} else if ev.Key() == tcell.KeyRight {
					moveCursor(1, 0)
				} else if ev.Key() == tcell.KeyEnter {
					moveCursor(0, 1)
				} else if ev.Key() == tcell.KeyCtrlS {
					// Request formatting
					if err := sendFormattingRequest(stdin, "file://"+currentFile); err != nil {
						logf("Error sending formatting request: %v", err)
					}
				} else {
					if ev.Rune() == '.' {
						// Request completion
						if resp, err := sendCompletionRequest(stdin, cy, cx); err != nil {
							logf("Error sending completion request: %v", err)
						} else {
							for _, v := range resp.Result.Items {
								marshal, err := json.Marshal(v.TextEdit)
								poe(err)
								logf("%s %s %s", v.Label, v.Detail, marshal)
							}
						}
					} else {
						// Insert the new rune at the current cursor position and shift others to the right
						if ev.Key() == 127 {

						} else {
							newRune := ev.Rune()
							if newRune != 0 { // Ensure it's a valid rune
								editorArea.content.InsertChar(cy, cx, newRune, CODE_DEFAULT_STYLE)
								moveCursor(0, 0) // Move the cursor to the right after inserting
							}
							// Print the key code
							logf("Key: %v", ev.Key())
						}

					}
				}
			}
		case *tcell.EventMouse:
			//x, y := ev.Position()
			//
			//switch ev.Buttons() {
			//case tcell.Button1, tcell.Button2:
			//if ox < 0 {
			//	ox, oy = x, y // record location when click started
			//}

			//case tcell.ButtonNone:
			//	if ox >= 0 {
			//		label := fmt.Sprintf("%d,%d to %d,%d", ox, oy, x, y)
			//		drawBox(s, ox, oy, x, y, boxStyle, label)
			//		ox, oy = -1, -1
			//	}
		}
	}
}

func enableMenu(enabled bool) {
	if !enabled {
		menuArea.FillStyle(MENU_DISABLED_STYLE)
		menuState = "disabled"
	} else {
		menuArea.FillStyle(MENU_ENABLED_STYLE)
		menuState = "enabled"
	}
}

func NewWideLineThing(x, y int, s tcell.Style, content string) *ViewArea {
	sw, _ := screen.Size()
	txt := NewEditor()

	v := &ViewArea{
		x:          x,
		y:          y,
		w:          sw,
		h:          1,
		scrollable: false,
		multiline:  false,
		editable:   false,
		content:    txt,
	}
	v.placeText(0, 0, content, s)
	v.dirty = true
	return v
}

func NewEditor() editors.Editor {
	return &editors.DirtSimpleEditor{}
}

func setupAreas() {
	width, height := screen.Size()
	editorArea = &ViewArea{
		x:          0,
		y:          2,
		w:          width,
		h:          height,
		scrollable: true,
		multiline:  true,
		editable:   true,
		content:    NewEditor(),
	}
	logArea = &ViewArea{
		x:         0,
		y:         height - 5,
		w:         width,
		h:         5,
		multiline: true,
		content:   NewEditor(),
	}
	menuArea = NewWideLineThing(0, 0, MENU_DISABLED_STYLE, "Q)uit T)ools R)efactor S)earch")
	tabsArea = NewWideLineThing(0, 1, FILE_TAB_STYLE, "File Tabs")
}

func (va *ViewArea) placeText(line int, pos int, msg string, style tcell.Style) {
	va.dirty = true
	if va.content == nil {
		va.content = NewEditor()
	}
	va.content.InsertText(line, pos, msg, style)
}

func (va *ViewArea) FillStyle(style tcell.Style) {
	if va.content != nil {
		currentLine := va.topVisibleLine
		totalLines := va.content.Length()
		for {
			if currentLine >= totalLines {
				break
			}
			line, _ := va.content.GetLine(currentLine)
			length := len(line)
			va.content.ApplyStyle(currentLine, 0, length, style)
			currentLine++
		}
		va.dirty = true
	}
}

func loadFiles() {
	cwd, err := os.Getwd()
	poe(err)

	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the TextDocument ends with .go and not in a vendor directory
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.Contains(path, "vendor") {
			// Construct the relative path from the current working directory to the TextDocument
			relPath, err := filepath.Rel(cwd, path)
			poe(err) // Handle any potential error while determining the relative path

			loadFile(relPath) // Use the relative path instead of the absolute one
		}
		return nil
	})

	poe(err)
}

func moveCursor(dx, dy int) {
	//return
	//width, height := screen.Size()
	//f := files[currentFile]
	//nx := cx + dx
	//ny := cy + dy
	//if ny >= len(f.lines) || ny < 0 {
	//	logf("Invalid cursor YY position: %d, %d", nx, ny)
	//	return
	//}
	//line := f.lines[ny]
	//if nx < 0 {
	//	logf("Invalid cursor XX position: %d, %d %s", nx, ny, line)
	//	return
	//}
	//if nx >= len(line) {
	//	nx = len(line)
	//}
	//
	//x := nx + LINE_NUMBERS_WIDTH
	//y := ny + EDITOR_LINE
	//if x >= width || y >= height-NUM_LOG_LINES {
	//	logf("Invalid cursor XX,YY position: %d, %d %s", x, y, line)
	//	return
	//}
	//
	//cx = nx
	//cy = ny
	//
	//msg := ""
	//for i := 0; i < 5 && i < len(line); i++ {
	//	//	cell := line[i]
	//	//msg += string(cell.rune)
	//}
	//drawText(60, 0, CODE_DEFAULT_STYLE, "%10s", msg)
	////if cx >= len(line) {
	////	cx = len(line) - 1
	////	x = cx + LINE_NUMBERS_WIDTH
	////}
	//if cx < 0 || cy < 0 {
	//	poe(fmt.Errorf("invalid cursor position: %d, %d", cx, cy))
	//}
	//drawText(50, 0, CODE_DEFAULT_STYLE, "(%3d,%3d)", cx, cy)
	//screen.ShowCursor(x, y)
}

func drawText(x, y int, style tcell.Style, format string, args ...any) {
	txt := fmt.Sprintf(format, args...)
	for i, r := range txt {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

func loadFile(filePath string) {
	// Read TextDocument content
	content, err := os.ReadFile(filePath)
	poe(err)
	f := NewEditor()

	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Text()
		f.InsertLine(lineNumber, line)
		lineNumber++
	}
	if err := scanner.Err(); err != nil {
		poe(err) // Handle any potential scanning errors
	}
	files[filePath] = f
}

func drawFileTabs() {
	sortedNames := make([]string, 0, len(files))
	for name := range files {
		sortedNames = append(sortedNames, name)
	}

	cx := 0
	for i, name := range sortedNames {
		style := FILE_TAB_STYLE
		if name == currentFile {
			style = style.Reverse(true)
		}
		msg := fmt.Sprintf("%d) %-15s ", i+1, name[:min(len(name), 15)])
		tabsArea.placeText(0, cx, msg, style)
		cx += 20
	}

}

//
//func drawFile(line int, filePath string) {
//
//	f, ok := files[filePath]
//	if !ok {
//		logf("File not found: %s", filePath)
//		return
//	}
//
//	currentFile = filePath
//
//	// Calculate the drawing range
//	_, screenHeight := screen.Size()
//	startLine := EDITOR_LINE
//	endLine := screenHeight - NUM_LOG_LINES
//
//	// Walk the internal buffer of the TextDocument and draw text line by line
//	for i := line; i < len(f.lines) && (startLine+i) < endLine; i++ {
//		var lineContent string
//		for _, cell := range f.lines[i] {
//			lineContent += string(cell)
//		}
//		drawText(LINE_NUMBERS_WIDTH, startLine+i, CODE_DEFAULT_STYLE, "%s", lineContent)
//	}
//
//	screen.SetTitle(filePath)
//	moveCursor(0, 0)
//}

func poe(err error) {
	if err != nil {
		panic(err)
	}
}

func initLsp(stdin io.WriteCloser) {
	if resp, err := sendInitializationRequest(stdin); err != nil {
		logf("Error sending initialization request: %v", err)
		return
	} else {
		logf("lsp: init done: %+v", resp)
	}

	if err := sendInitialized(stdin); err != nil {
		logf("Error sending Initialized request: %v", err)
		return
	}

	if resp, err := sendDidOpen(stdin, "testdata/testprogram.go"); err != nil {
		logf("Error sending initialization request: %v", err)
		return
	} else {
		logf("lsp: didopencomplete %+v", resp)
	}

	if resp, err := sendSyntax(stdin); err != nil {
		logf("Error sending syntax request: %v", err)
		return
	} else {
		logf("lsp: syntax %+v", resp)
	}
}

type NULL_PARAM_TYPE struct{}

var NO_PARAMS = NULL_PARAM_TYPE{}

func sendInitialized(stdin io.WriteCloser) error {
	rq := req[NULL_PARAM_TYPE]("initialized", NO_PARAMS)
	return sendAsync(stdin, rq)
}

func sendInitializationRequest(stdin io.Writer) (Response[lsp.InitializeResult], error) {

	// Create a document URI pointing to the testdata directory under the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return Response[lsp.InitializeResult]{}, fmt.Errorf("failed to get current working directory: %v", err)
	}
	testdataURI := lsp.DocumentURI("file://" + filepath.Join(cwd, "testdata"))

	p := lsp.InitializeParams{
		RootURI:      testdataURI,
		ClientInfo:   lsp.ClientInfo{},
		Capabilities: lsp.ClientCapabilities{},
		ProcessID:    os.Getpid(),
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
	logf("Sending request: %+v", r.Method)

	var rtn Response[RESP]
	defer logf("Response from %s %d", r.Method, rtn.ID)
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

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	body := fmt.Sprintf("%s", data)
	_, err = stdin.Write([]byte(header))
	if err != nil {
		logf("Error writing heqders to stdin: %v", err)
		return err
	}
	n, err := stdin.Write([]byte(body))
	if err != nil {
		logf("Error writing body to stdin: %v", err)
		return err
	}
	if n != len(body) {
		logf("Error writing body to stdin: %v", err)
		return fmt.Errorf("short write")
	}

	return nil
}

func sendSyntax(stdin io.Writer) (Response[lsp.SemanticHighlightingTokens], error) {
	r := lsp.TextDocumentIdentifier{
		URI: lsp.DocumentURI("file://" + currentFile),
	}
	rq := req[lsp.TextDocumentIdentifier]("textDocument/semanticTokens/full", r)
	return sendSync[lsp.TextDocumentIdentifier, lsp.SemanticHighlightingTokens](stdin, rq)
}

// Open a TextDocument and notify gopls about it
func sendDidOpen(stdinWriter io.Writer, filePath string) (rslt Response[NULL_PARAM_TYPE], err error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		err = fmt.Errorf("failed to read TextDocument: %v", err)
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path: %v", err)
		return
	}

	uri := lsp.DocumentURI("file://" + absPath)
	request := Request[lsp.DidOpenTextDocumentParams]{
		JsonRPC: "2.0",
		ID:      2,
		Method:  "textDocument/didOpen",
		Params: lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{
				URI:        uri,
				LanguageID: "go",
				Version:    1,
				Text:       string(content),
			},
		},
	}
	rslt, err = sendSync[lsp.DidOpenTextDocumentParams, NULL_PARAM_TYPE](stdinWriter, request)
	return
}

func listenForErrors(errPipe io.ReadCloser) {
	scanner := bufio.NewScanner(errPipe)
	for scanner.Scan() {
		logf("scanned error %s", scanner.Text())
	}
	if scanner.Err() != nil {
		logf("Error reading from gopls: %v", scanner.Err())
	} else {
		logf("done for errors on gopls")
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

func listenToGopls(stdout io.ReadCloser) {
	for {
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
			Params  map[string]interface{}
		}
		var resp = jsonRpcResponse{}
		err = json.Unmarshal(buffer, &resp)
		if err != nil {
			logf("Error parsing JSON-RPC response: %v", err)
			panic(err)
		}
		omlock.Lock()
		if listener, ok := outstandingMethods[resp.ID]; ok {
			omlock.Unlock()
			listener <- buffer
			continue
		}
		omlock.Unlock()
		switch resp.Method {
		case "window/showMessage":
			logf("Message: %s", resp.Params["message"])
			continue
		case "window/logMessage":
			logf("Log: %s", resp.Params["message"])
			continue
		case "textDocument/publishDiagnostics":
			logf("Diagnostics: %v", resp.Params)
			continue
		}

		logf("No listener for %d", resp.ID)

	}
	return
}

func drawBox(x1, y1, x2, y2 int, style tcell.Style, text string) {
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	if x2 < x1 {
		x1, x2 = x2, x1
	}

	// Fill background
	for row := y1; row <= y2; row++ {
		for col := x1; col <= x2; col++ {
			screen.SetContent(col, row, ' ', nil, style)
		}
	}

	// Draw borders
	for col := x1; col <= x2; col++ {
		screen.SetContent(col, y1, tcell.RuneHLine, nil, style)
		screen.SetContent(col, y2, tcell.RuneHLine, nil, style)
	}
	for row := y1 + 1; row < y2; row++ {
		screen.SetContent(x1, row, tcell.RuneVLine, nil, style)
		screen.SetContent(x2, row, tcell.RuneVLine, nil, style)
	}

	// Only draw corners if necessary
	if y1 != y2 && x1 != x2 {
		screen.SetContent(x1, y1, tcell.RuneULCorner, nil, style)
		screen.SetContent(x2, y1, tcell.RuneURCorner, nil, style)
		screen.SetContent(x1, y2, tcell.RuneLLCorner, nil, style)
		screen.SetContent(x2, y2, tcell.RuneLRCorner, nil, style)
	}

	printText(x1+1, y1+1, x2-1, y2-1, style, text)
}

func printText(x1, y1, x2, y2 int, style tcell.Style, text string) {
	row := y1
	col := x1
	for _, r := range []rune(text) {
		screen.SetContent(col, row, r, nil, style)
		col++
		if col >= x2 {
			row++
			col = x1
		}
		if row > y2 {
			break
		}
	}
}

func sendCompletionRequest(stdin io.Writer, line, character int) (Response[CompletionList], error) {

	absPath, err := filepath.Abs(currentFile)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path: %v", err)
		panic(err)
	}

	uri := lsp.DocumentURI("file://" + absPath)

	r := lsp.CompletionParams{
		TextDocumentPositionParams: lsp.TextDocumentPositionParams{
			TextDocument: lsp.TextDocumentIdentifier{
				URI: uri,
			},
			Position: lsp.Position{
				Line:      line,
				Character: character,
			},
		},
		Context: lsp.CompletionContext{
			TriggerKind:      lsp.CTKInvoked,
			TriggerCharacter: ".",
		},
	}
	rq := req[lsp.CompletionParams]("textDocument/completion", r)
	return sendSync[lsp.CompletionParams, CompletionList](stdin, rq)
}

type Documentation struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type CompletionItem struct {
	Label            string                 `json:"label"`
	Kind             lsp.CompletionItemKind `json:"kind,omitempty"`
	Detail           string                 `json:"detail,omitempty"`
	Documentation    Documentation          `json:"documentation,omitempty"`
	SortText         string                 `json:"sortText,omitempty"`
	FilterText       string                 `json:"filterText,omitempty"`
	InsertText       string                 `json:"insertText,omitempty"`
	InsertTextFormat lsp.InsertTextFormat   `json:"insertTextFormat,omitempty"`
	TextEdit         *lsp.TextEdit          `json:"textEdit,omitempty"`
	Data             interface{}            `json:"data,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
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

// Position is a convenience struct for cursor/selection endpoints.
type Position struct {
	Line   int
	Column int
}
