package rope

import (
	"github.com/gdamore/tcell/v2"
	"testing"
)

// Unit Tests
func TestRopeInsertAndRetrieveStyle(t *testing.T) {
	rope := NewRope()
	redStyle := tcell.StyleDefault.Foreground(tcell.ColorRed)
	blueStyle := tcell.StyleDefault.Foreground(tcell.ColorBlue)

	rope.Insert(0, 10, redStyle)
	rope.Insert(10, 5, blueStyle)

	if rope.GetStyle(5) != redStyle {
		t.Errorf("Expected red style at position 5")
	}

	if rope.GetStyle(12) != blueStyle {
		t.Errorf("Expected blue style at position 12")
	}
}

func TestRopeApplyStyle(t *testing.T) {
	rope := NewRope()
	redStyle := tcell.StyleDefault.Foreground(tcell.ColorRed)
	greenStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen)

	rope.Insert(0, 15, redStyle)
	rope.ApplyStyle(5, 5, greenStyle)

	if rope.GetStyle(5) != greenStyle {
		t.Errorf("Expected green style at position 5")
	}

	if rope.GetStyle(10) != greenStyle {
		t.Errorf("Expected green style at position 10")
	}

	if rope.GetStyle(12) != redStyle {
		t.Errorf("Expected red style at position 12")
	}
}

func TestRopeStylePersistence(t *testing.T) {
	rope := NewRope()
	blueStyle := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	rope.Insert(0, 20, blueStyle)

	if rope.GetStyle(15) != blueStyle {
		t.Errorf("Expected blue style at position 15")
	}
}
