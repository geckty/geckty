package vt

import (
	"io"
	"testing"
)

func TestTakeBellAfterBEL(t *testing.T) {
	term := New(10, 2, io.Discard, nil, 0)
	if term.TakeBell() {
		t.Fatal("TakeBell should be false before any BEL")
	}
	term.Parse([]byte("\a"))
	if !term.TakeBell() {
		t.Fatal("TakeBell should be true after BEL")
	}
	if term.TakeBell() {
		t.Fatal("TakeBell should clear the pending flag")
	}
}
