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

func (d *DirtSimpleEditor) SetLine(line int, text string, style ...tcell.Style) {

	if line < 0 {
		panic("line index out of range")
	} else if line >= len(d.lines) {
		d.InsertLine(line, text, style...)
		return
	}
	styledLine := makeStyledLine(style, text)
	d.lines[line] = styledLine

}

// InsertLine  insert a line of text into the editor, shifting lines that are below that down
// and if a style is provided we should apply that style to the characters of the text
func (d *DirtSimpleEditor) InsertLine(line int, text string, style ...tcell.Style) {
	if line < 0 || line > len(d.lines)+1 {
		panic("line index out of range")
	}

	styledLine := makeStyledLine(style, text)

	d.lines = slices.Insert(d.lines, line, styledLine)
}

func makeStyledLine(style []tcell.Style, text string) StyledLine {
	// Prepare the StyledLine
	var styledLine StyledLine
	if len(style) > 1 {
		// Apply the provided style to each character in the text
		idx := 0
		for _, char := range text {
			styledLine = append(styledLine, StyledChar{
				Char:  char,
				Style: style[idx],
			})
			idx++
		}
	} else if len(style) == 1 {
		// Use default style if none is provided
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
	return styledLine
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

// InsertChar inserts a character at a specified position in the text editor.
// If the character is '\n', it splits the current line into two, with the part
// before the cursor remaining on the current line and the part after the cursor
// moved to a new line below.  Returns true if the line needs to be 100% redrawn
func (d *DirtSimpleEditor) InsertChar(line int, column int, text rune, style tcell.Style) {
	if text == '\n' {
		sl := d.lines[line]
		d.lines[line] = sl[:column]                           // trim the current line to only hold the before \n
		d.lines = slices.Insert(d.lines, line+1, sl[column:]) // and the stuff after the cursor goes on the next line
		//	log.Printf("line: %d, column: %d, text: %s, style: %s\n", line, column, text, style)
	} else {
		runes, styles := d.GetLine(line)
		runes = slices.Insert(runes, column, text)
		styles = slices.Insert(styles, column, style)
		d.SetLine(line, string(runes), styles...)
	}
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
