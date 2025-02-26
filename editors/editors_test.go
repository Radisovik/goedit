package editors

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDirtSimple(t *testing.T) {
	ds := NewDirtSimpleEditor()

	ds.InsertLine(0, "foo")
	ds.InsertLine(1, "bar")
	ds.InsertLine(0, "baz")

	line, _ := ds.GetLine(0)
	assert.Equal(t, "baz", toString(line), "Should be baz")

	line, _ = ds.GetLine(1)
	assert.Equal(t, "foo", toString(line), "Should be foo")

	line, _ = ds.GetLine(2)
	assert.Equal(t, "bar", toString(line), "Should be bar")

}

func toString(chars []rune) string {
	return string(chars)
}
