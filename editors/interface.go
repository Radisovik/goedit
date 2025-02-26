package editors

import "github.com/gdamore/tcell/v2"

type Editor interface {
	InsertLine(line int, text string, style ...tcell.Style)
	InsertChar(line int, column int, text rune, style tcell.Style)
	DeleteLine(line int)
	DeleteChar(line int, column int)
	Subscribe(line int, column int, height int, width int, callback func(line int, column int, char rune, style tcell.Style)) int
	ApplyStyle(line int, column int, length int, style tcell.Style)
	Unsubscribe(id int)
	Undo()
	Redo()
	GetLine(line int) ([]rune, []tcell.Style)
	InsertText(line int, pos int, msg string, style tcell.Style)
	Length() int
}
