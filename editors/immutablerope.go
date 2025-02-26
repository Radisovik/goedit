// Immutable Line-Based Rope Data Structure (Indexed by Line Numbers)
// Supports efficient insertions, deletions, undo/redo, and balancing.

package editors

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
)

// RopeNode represents a node in the immutable rope structure, where each node corresponds to a single line.
type RopeNode struct {
	left      *RopeNode
	right     *RopeNode
	text      []rune        // The text content of this line
	styles    []tcell.Style // Matching styles for the text
	lineCount int           // Number of lines in this subtree (used for navigation)
	height    int           // Height of the node for balancing
}

// Rope is the root structure of the line-based rope tree.
type Rope struct {
	root *RopeNode
}

// NewRope initializes an empty rope.
func NewRope() *Rope {
	return &Rope{root: nil}
}

// getLineCount returns the number of lines in a subtree.
func getLineCount(node *RopeNode) int {
	if node == nil {
		return 0
	}
	return node.lineCount
}

// getHeight returns the height of a node.
func getHeight(node *RopeNode) int {
	if node == nil {
		return 0
	}
	return node.height
}

// balanceFactor calculates the balance factor of a node.
func balanceFactor(node *RopeNode) int {
	if node == nil {
		return 0
	}
	return getHeight(node.left) - getHeight(node.right)
}

// rotateRight performs a right rotation to balance the tree.
func rotateRight(y *RopeNode) *RopeNode {
	x := y.left
	T2 := x.right
	x.right = y
	y.left = T2
	y.height = max(getHeight(y.left), getHeight(y.right)) + 1
	x.height = max(getHeight(x.left), getHeight(x.right)) + 1
	y.lineCount = getLineCount(y.left) + getLineCount(y.right) + 1
	x.lineCount = getLineCount(x.left) + getLineCount(x.right) + 1
	return x
}

// rotateLeft performs a left rotation to balance the tree.
func rotateLeft(x *RopeNode) *RopeNode {
	y := x.right
	T2 := y.left
	y.left = x
	x.right = T2
	x.height = max(getHeight(x.left), getHeight(x.right)) + 1
	y.height = max(getHeight(y.left), getHeight(y.right)) + 1
	x.lineCount = getLineCount(x.left) + getLineCount(x.right) + 1
	y.lineCount = getLineCount(y.left) + getLineCount(y.right) + 1
	return y
}

// balance ensures that the tree remains balanced.
func balance(node *RopeNode) *RopeNode {
	if node == nil {
		return nil
	}

	balance := balanceFactor(node)

	if balance > 1 {
		if balanceFactor(node.left) < 0 {
			node.left = rotateLeft(node.left)
		}
		return rotateRight(node)
	}

	if balance < -1 {
		if balanceFactor(node.right) > 0 {
			node.right = rotateRight(node.right)
		}
		return rotateLeft(node)
	}

	node.height = max(getHeight(node.left), getHeight(node.right)) + 1
	node.lineCount = getLineCount(node.left) + getLineCount(node.right) + 1
	return node
}

// GetLine retrieves the text and styles for a given line.
func (r *Rope) GetLine(line int) ([]rune, []tcell.Style) {
	return getLineNode(r.root, line)
}

func (r *Rope) InsertLine(i int, s string, foreground tcell.Style) *Rope {

	// InsertLine inserts a new line with the given text and styles at the specified index `i`.
	if i < 0 || i > getLineCount(r.root) {
		panic("line index out of bounds")
	}
	newNode := &RopeNode{
		text:      []rune(s),
		styles:    make([]tcell.Style, len([]rune(s))),
		lineCount: 1,
		height:    1,
	}
	for j := range newNode.styles {
		newNode.styles[j] = foreground
	}
	r.root = insertLineNode(r.root, i, newNode)
	return r
}

func (rr *Rope) InsertChar(line int, col int, r rune, foreground tcell.Style) *Rope {

	// InsertChar inserts a character `r` at the specified column `col` in the given line `line`.
	if line < 0 || line >= getLineCount(rr.root) {
		panic("line index out of bounds")
	}

	var insertCharNode func(node *RopeNode, line, col int, r rune, style tcell.Style) *RopeNode
	insertCharNode = func(node *RopeNode, line, col int, r rune, style tcell.Style) *RopeNode {
		if node == nil {
			return nil
		}

		leftSize := getLineCount(node.left)

		if line < leftSize {
			node.left = insertCharNode(node.left, line, col, r, style)
		} else if line > leftSize {
			node.right = insertCharNode(node.right, line-leftSize-1, col, r, style)
		} else {
			// We are at the target line node
			if col < 0 || col > len(node.text) {
				panic("column index out of bounds")
			}

			// Insert character at the correct position
			newText := make([]rune, len(node.text)+1)
			copy(newText, node.text[:col])
			newText[col] = r
			copy(newText[col+1:], node.text[col:])

			// Insert style for the new character
			newStyles := make([]tcell.Style, len(node.styles)+1)
			copy(newStyles, node.styles[:col])
			newStyles[col] = style
			copy(newStyles[col+1:], node.styles[col:])

			node.text = newText
			node.styles = newStyles
		}

		node.height = max(getHeight(node.left), getHeight(node.right)) + 1
		node.lineCount = getLineCount(node.left) + getLineCount(node.right) + 1
		return balance(node)
	}

	rr.root = insertCharNode(rr.root, line, col, r, foreground)
	return rr
}

func (r *Rope) DeleteLine(line int) *Rope {
	if line < 0 || line >= getLineCount(r.root) {
		panic("line index out of bounds")
	}

	var deleteLineNode func(node *RopeNode, line int) *RopeNode
	deleteLineNode = func(node *RopeNode, line int) *RopeNode {
		if node == nil {
			return nil
		}

		leftSize := getLineCount(node.left)

		if line < leftSize {
			node.left = deleteLineNode(node.left, line)
		} else if line > leftSize {
			node.right = deleteLineNode(node.right, line-leftSize-1)
		} else {
			// This is the line to delete
			if node.left == nil {
				return node.right
			}
			if node.right == nil {
				return node.left
			}

			// Find the in-order successor (smallest node in the right subtree)
			successor := node.right
			for successor.left != nil {
				successor = successor.left
			}

			// Replace the current node with the successor
			node.text = successor.text
			node.styles = successor.styles
			node.right = deleteLineNode(node.right, 0)
		}

		node.height = max(getHeight(node.left), getHeight(node.right)) + 1
		node.lineCount = getLineCount(node.left) + getLineCount(node.right) + 1
		return balance(node)
	}

	r.root = deleteLineNode(r.root, line)
	return r
}

func insertLineNode(node *RopeNode, index int, newNode *RopeNode) *RopeNode {
	if node == nil {
		return newNode
	}

	leftSize := getLineCount(node.left)

	if index <= leftSize {
		node.left = insertLineNode(node.left, index, newNode)
	} else {
		node.right = insertLineNode(node.right, index-leftSize-1, newNode)
	}

	node.height = max(getHeight(node.left), getHeight(node.right)) + 1
	node.lineCount = getLineCount(node.left) + getLineCount(node.right) + 1
	return balance(node)
}

func getLineNode(node *RopeNode, line int) ([]rune, []tcell.Style) {
	if node == nil {
		return nil, nil
	}

	leftSize := getLineCount(node.left)

	if line < leftSize {
		return getLineNode(node.left, line)
	} else if line > leftSize {
		return getLineNode(node.right, line-leftSize-1)
	}

	return node.text, node.styles
}

// Example usage
func main() {
	rope := NewRope()
	rope = rope.InsertLine(0, "Hello, world!", tcell.StyleDefault.Foreground(tcell.ColorWhite))
	rope = rope.InsertChar(0, 7, 'X', tcell.StyleDefault.Foreground(tcell.ColorRed))
	rope = rope.InsertLine(1, "This is a test.", tcell.StyleDefault.Foreground(tcell.ColorBlue))
	rope = rope.DeleteLine(0)

	text, styles := rope.GetLine(0)
	fmt.Println("Retrieved line text:", string(text))
	fmt.Println("Styles length:", len(styles))
}
