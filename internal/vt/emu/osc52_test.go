package emu

// Tests for the geckty-added OSC52Handler hook (see module.go's
// OSC52Handler doc comment and parse.go's handleOSC52). Kept in its own
// file, separate from upstream's test files, since this behavior isn't
// part of upstream cy/pkg/emu.

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeOSC52 struct {
	writes map[string][]byte
	reads  map[string][]byte
}

func newFakeOSC52() *fakeOSC52 {
	return &fakeOSC52{writes: map[string][]byte{}, reads: map[string][]byte{}}
}

func (f *fakeOSC52) OSC52Write(pc string, data []byte) {
	f.writes[pc] = append([]byte(nil), data...)
}

func (f *fakeOSC52) OSC52Read(pc string) ([]byte, bool) {
	d, ok := f.reads[pc]
	return d, ok
}

func TestOSC52WriteCallsHandler(t *testing.T) {
	handler := newFakeOSC52()
	term := New(WithOSC52Handler(handler))

	payload := base64.StdEncoding.EncodeToString([]byte("hello clipboard"))
	_, err := term.Write([]byte("\x1b]52;c;" + payload + "\x07"))
	require.NoError(t, err)

	require.Equal(t, []byte("hello clipboard"), handler.writes["c"])
}

func TestOSC52WriteWithoutHandlerDoesNotPanic(t *testing.T) {
	term := New() // no OSC52Handler installed
	payload := base64.StdEncoding.EncodeToString([]byte("hi"))
	_, err := term.Write([]byte("\x1b]52;c;" + payload + "\x07"))
	require.NoError(t, err)
}

func TestOSC52QueryRespondsThroughWriter(t *testing.T) {
	handler := newFakeOSC52()
	handler.reads["c"] = []byte("clipboard contents")

	var out bytes.Buffer
	term := New(WithWriter(&out), WithOSC52Handler(handler))

	_, err := term.Write([]byte("\x1b]52;c;?\x07"))
	require.NoError(t, err)

	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("clipboard contents")) + "\x1b\\"
	require.Equal(t, want, out.String())
}

func TestOSC52QueryWithNoDataSendsNoResponse(t *testing.T) {
	handler := newFakeOSC52() // reads["c"] intentionally unset -> ok=false

	var out bytes.Buffer
	term := New(WithWriter(&out), WithOSC52Handler(handler))

	_, err := term.Write([]byte("\x1b]52;c;?\x07"))
	require.NoError(t, err)
	require.Empty(t, out.String())
}

func TestOSC52QueryWithoutHandlerSendsNoResponse(t *testing.T) {
	var out bytes.Buffer
	term := New(WithWriter(&out)) // no OSC52Handler installed

	_, err := term.Write([]byte("\x1b]52;c;?\x07"))
	require.NoError(t, err)
	require.Empty(t, out.String())
}

func TestOSC52IgnoresNonClipboardSelector(t *testing.T) {
	// Only "c" (clipboard) is supported; "p" (primary) and "s" (select)
	// are recognized syntax but deliberately not forwarded to the
	// handler.
	handler := newFakeOSC52()
	term := New(WithOSC52Handler(handler))

	payload := base64.StdEncoding.EncodeToString([]byte("primary selection"))
	_, err := term.Write([]byte("\x1b]52;p;" + payload + "\x07"))
	require.NoError(t, err)

	require.Empty(t, handler.writes)
}

func TestOSC52DoesNotCorruptGridState(t *testing.T) {
	// Matches the M0 spike's spirit for APC: an OSC sequence must never
	// leak into the printable grid, regardless of whether a handler is
	// installed.
	handler := newFakeOSC52()
	term := New(WithOSC52Handler(handler))

	payload := base64.StdEncoding.EncodeToString([]byte("clipboard data"))
	_, err := term.Write([]byte("\x1b]52;c;" + payload + "\x07hi"))
	require.NoError(t, err)

	require.Equal(t, 'h', term.Cell(0, 0).Char)
	require.Equal(t, 'i', term.Cell(1, 0).Char)
}
