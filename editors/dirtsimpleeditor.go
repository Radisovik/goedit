package editors

import (
	"github.com/gdamore/tcell/v2"
	"slices"
)

func NewDirtSimpleEditor() Editor {
	rtn := &DirtSimpleEditor{}
	return rtn
}

type DirtSimpleEditor struct {
	// lines represents the content of the editor, where each string is a single line of text.
	lines []StyledLine
}

type StyledLine []StyledChar

type StyledChar struct {
	Char  rune
	Style tcell.Style
}

func (d *DirtSimpleEditor) Length() int {
	return len(d.lines)
}

func (d *DirtSimpleEditor) InsertText(line int, pos int, msg string, style tcell.Style) {
	d.DeleteLine(line)
	d.InsertLine(line, msg, style)
}

// InsertLine  insert a line of text into the editor, shifting lines that are below that down
// and if a style is provided we should apply that style to the characters of the text
func (d *DirtSimpleEditor) InsertLine(line int, text string, style ...tcell.Style) {
	if line < 0 || line > len(d.lines)+1 {
		panic("line index out of range")
	}

	// Prepare the StyledLine
	var styledLine StyledLine
	if len(style) > 0 {
		// Apply the provided style to each character in the text
		for _, char := range text {
			styledLine = append(styledLine, StyledChar{
				Char:  char,
				Style: style[0],
			})
		}
	} else {
		// Use default style if none is provided
		for _, char := range text {
			styledLine = append(styledLine, StyledChar{
				Char:  char,
				Style: tcell.StyleDefault,
			})
		}
	}

	d.lines = slices.Insert(d.lines, line, styledLine)
	//
	//// Create a new slice that is 1 bigger
	//newLines := make([]StyledLine, len(d.lines)+1)
	//
	//if line == len(d.lines) {
	//	copy(newLines, d.lines)
	//	newLines[line] = styledLine
	//} else if line == 0 {
	//	copy(newLines[1:], d.lines)
	//	newLines[0] = styledLine
	//} else {
	//
	//	idx := 0
	//	for {
	//		if idx == line {
	//			break
	//		}
	//		newLines[idx] = d.lines[idx]
	//		idx++
	//	}
	//	newLines[line] = styledLine
	//
	//	for {
	//		if idx == len(newLines) {
	//			break
	//		}
	//		newLines[idx+1] = d.lines[idx]
	//	}
	//
	//}
	//
	//// Replace the original lines with the newLines
	//d.lines = newLines
}

func (d *DirtSimpleEditor) GetLine(line int) ([]rune, []tcell.Style) {
	if line < 0 {
		panic("line index out of range")
	}
	var runes []rune
	var styles []tcell.Style
	if line < len(d.lines) {
		for _, styledChar := range d.lines[line] {
			runes = append(runes, styledChar.Char)
			styles = append(styles, styledChar.Style)
		}
	}
	return runes, styles
}

func (d *DirtSimpleEditor) InsertChar(line int, column int, text rune, style tcell.Style) {
	runes, styles := d.GetLine(line)
	runes = slices.Insert(runes, column, text)
	styles = slices.Insert(styles, column, style)
	d.DeleteLine(line)
	d.InsertLine(line, string(runes), styles...)
}

// DeleteLine remove the line of text, along with the styles, shifting lines and styles up
func (d *DirtSimpleEditor) DeleteLine(line int) {
	if line < 0 {
		panic("line index out of range")
	}
	if len(d.lines) == 0 {
		return
	}
	if len(d.lines) == 1 && line == 0 {
		d.lines = []StyledLine{}
		return
	}

	d.lines = slices.Delete(d.lines, line, line)
}

func (d *DirtSimpleEditor) DeleteChar(line int, column int) {

	if line < 0 || line >= len(d.lines) {
		panic("line index out of range")
	}
	if column < 0 || column >= len(d.lines[line]) {
		panic("column index out of range")
	}

	// Remove the character at the specified column
	d.lines[line] = append(d.lines[line][:column], d.lines[line][column+1:]...)

	// If this was the only character on the line, delete the entire line
	if len(d.lines[line]) == 0 {
		d.DeleteLine(line)
	}
}

func (d *DirtSimpleEditor) Subscribe(line int, column int, height int, width int, callback func(line int, column int, char rune, style tcell.Style)) int {
	//TODO implement me
	panic("implement me")
}

func (d *DirtSimpleEditor) ApplyStyle(line int, column int, length int, style tcell.Style) {

}

func (d *DirtSimpleEditor) Unsubscribe(id int) {
	//TODO implement me
	panic("implement me")
}

func (d *DirtSimpleEditor) Undo() {
	//TODO implement me
	panic("implement me")
}

func (d *DirtSimpleEditor) Redo() {
	//TODO implement me
	panic("implement me")
}
