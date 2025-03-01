package main

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
)
import "github.com/gdamore/tcell/v2/views"

func main() {
	screen, err := tcell.NewScreen()
	poe(err)

	ap := views.Application{}
	ap.SetScreen(screen)

	panels := views.NewPanel()
	area := views.NewTextArea()
	panels.SetContent(area)
	menu := views.NewTextBar()
	menu.SetCenter("menu", tcell.StyleDefault)
	menu.SetRight("right", tcell.StyleDefault)
	menu.SetLeft("left", tcell.StyleDefault)

	text := views.NewText()
	text.SetText("title")
	text.SetAlignment(views.AlignMiddle)
	panels.SetTitle(text)

	status := views.NewText()
	status.SetText("status")
	panels.SetStatus(status)

	area.SetContent("blah blah]\neat\nool")
	area.SetModel(Thing{})

	panels.SetMenu(menu)
	ap.SetRootWidget(panels)

	err = ap.Run()
	poe(err)
}

type Thing struct {
}

var odd = tcell.Style{}.Foreground(tcell.ColorRed).Background(tcell.ColorWhite)
var even = tcell.Style{}.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)

func (t Thing) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	if y%2 == 0 {
		r := []rune(fmt.Sprintf("%d", x))[x]
		return r, even, nil, 0
	}
	return 'o', odd, nil, 0
}

func (t Thing) GetBounds() (int, int) {
	return 25, 25
}

func (t Thing) SetCursor(i int, i2 int) {

}

func (t Thing) GetCursor() (int, int, bool, bool) {
	return 5, 5, false, false
}

func (t Thing) MoveCursor(offx, offy int) {

}

func poe(err error) {
	if err != nil {
		panic(err)
	}
}
