package rope

// Rope Data Structure for Storing tcell Styles in GoEdit
// This implementation efficiently manages incremental syntax highlighting updates.

import (
	"github.com/gdamore/tcell/v2"
)

// RopeNode represents a node in the rope structure for styles
// Each node stores a segment of styles and its length.
type RopeNode struct {
	length int         // Number of characters this node represents
	style  tcell.Style // Style applied to this segment
	left   *RopeNode
	right  *RopeNode
	height int // Balance factor for AVL-like balancing
}

// Rope represents the root of the style rope
type Rope struct {
	root *RopeNode
}

// NewRope initializes an empty rope
func NewRope() *Rope {
	return &Rope{root: nil}
}

// Insert inserts a new style segment into the rope at a given offset
func (r *Rope) Insert(offset int, length int, style tcell.Style) {
	r.root = insertNode(r.root, offset, length, style)
}
func insertNode(node *RopeNode, offset, length int, style tcell.Style) *RopeNode {
	if node == nil {
		return &RopeNode{length: length, style: style, height: 1}
	}

	if offset < node.length {
		// Split existing node
		leftPart := &RopeNode{length: offset, style: node.style, height: 1}
		newNode := &RopeNode{length: length, style: style, height: 1}
		rightPart := &RopeNode{length: node.length - offset, style: node.style, height: 1}

		return balance(&RopeNode{
			length: node.length + length,
			left:   balance(leftPart),
			right:  balance(&RopeNode{length: rightPart.length, style: rightPart.style, left: newNode, right: rightPart}),
		})
	} else {
		node.right = insertNode(node.right, offset-node.length, length, style)
	}

	return balance(node)
}

// ApplyStyle modifies the style for a specific range
func (r *Rope) ApplyStyle(offset int, length int, style tcell.Style) {
	applyStyleToNode(r.root, offset, length, style)
}

func applyStyleToNode(node *RopeNode, offset, length int, style tcell.Style) {
	if node == nil {
		return
	}

	if offset < node.length {
		applyStyleToNode(node.left, offset, length, style)
	} else {
		applyStyleToNode(node.right, offset-node.length, length, style)
	}

	if offset <= node.length && offset+length > node.length {
		node.style = style
	}
}

// GetStyle retrieves the style at a specific character position
func (r *Rope) GetStyle(offset int) tcell.Style {
	return getStyleFromNode(r.root, offset)
}
func getStyleFromNode(node *RopeNode, offset int) tcell.Style {
	if node == nil {
		return tcell.StyleDefault
	}

	if offset < node.length {
		return getStyleFromNode(node.left, offset)
	} else if offset < node.length+node.right.length {
		return node.style // If we are within this node, return its style
	}

	return getStyleFromNode(node.right, offset-node.length)
}

// Balancing functions for AVL-like self-balancing
func height(node *RopeNode) int {
	if node == nil {
		return 0
	}
	return node.height
}

func balanceFactor(node *RopeNode) int {
	if node == nil {
		return 0
	}
	return height(node.left) - height(node.right)
}

func balance(node *RopeNode) *RopeNode {
	if node == nil {
		return nil
	}

	node.height = max(height(node.left), height(node.right)) + 1

	if balanceFactor(node) > 1 {
		if balanceFactor(node.left) < 0 {
			node.left = rotateLeft(node.left)
		}
		return rotateRight(node)
	}

	if balanceFactor(node) < -1 {
		if balanceFactor(node.right) > 0 {
			node.right = rotateRight(node.right)
		}
		return rotateLeft(node)
	}

	return node
}

func rotateRight(y *RopeNode) *RopeNode {
	x := y.left
	T2 := x.right
	x.right = y
	y.left = T2

	y.height = max(height(y.left), height(y.right)) + 1
	x.height = max(height(x.left), height(x.right)) + 1

	return x
}

func rotateLeft(x *RopeNode) *RopeNode {
	y := x.right
	T2 := y.left
	y.left = x
	x.right = T2

	x.height = max(height(x.left), height(x.right)) + 1
	y.height = max(height(y.left), height(y.right)) + 1

	return y
}
