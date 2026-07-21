// Package mouse encodes mouse wheel events into the escape sequences a
// shell expects when it has enabled mouse tracking (DECSET 1000/1002/1003/
// 1005/1006, mirrored by emu.ModeMouseButton/MouseMotion/MouseX10/MouseMany/
// MouseSgr).
package mouse

import (
	"fmt"

	"github.com/geckty/geckty/internal/vt/emu"
)

// Direction of a wheel tick.
type Direction int

// Wheel directions.
const (
	Up Direction = iota
	Down
)

// Modifiers is a toolkit-agnostic set of keyboard modifiers held during a
// wheel event. internal/protocol/* packages don't import gpucontext (only
// internal/ui/gogpu does) — callers translate from their UI toolkit's
// modifier type into this one at the boundary (see internal/ui/gogpu/app.go).
type Modifiers uint8

// Modifier bits.
const (
	ModShift Modifiers = 1 << iota
	ModAlt
	ModCtrl
)

func (m Modifiers) contain(o Modifiers) bool { return m&o == o }

// sgrWheelButton values per the SGR mouse protocol: 64 = wheel up, 65 =
// wheel down, combined with the same modifier bits button-press events use.
const (
	sgrWheelUp   = 64
	sgrWheelDown = 65
)

// legacyWheelBase is the X10/legacy mouse protocol's byte-encoded base for
// wheel buttons (button code 0x60, offset by 32 as legacy encoding
// requires, with +1 for wheel down).
const legacyBase = 0x60 + 32

// Button identifies which mouse button a press/release/drag involves.
type Button int

// Mouse buttons.
const (
	ButtonLeft Button = iota
	ButtonMiddle
	ButtonRight
)

// TrackingEnabled reports whether mode has any form of mouse tracking
// active (button, motion, X10, or "any-event" reporting) — the terminal
// needs some form of tracking enabled before wheel events should be
// encoded and sent to it at all, otherwise the shell just sees garbage
// bytes on its stdin.
func TrackingEnabled(mode emu.ModeFlag) bool {
	return mode&emu.ModeMouseMask != 0
}

// EncodeWheel returns the escape sequence for a wheel tick at 1-based
// terminal cell (col, row), honoring whichever mouse protocol mode is
// currently active (SGR if emu.ModeMouseSgr is set, legacy X10-style
// otherwise). ok is false if no tracking mode is active at all — callers
// should scroll local scrollback instead in that case (see
// session.Session.ScrollBy).
func EncodeWheel(mode emu.ModeFlag, dir Direction, col, row int, mods Modifiers) (out []byte, ok bool) {
	if !TrackingEnabled(mode) {
		return nil, false
	}

	if mode&emu.ModeMouseSgr != 0 {
		return encodeSGR(dir, col, row, mods), true
	}
	return encodeLegacy(dir, col, row), true
}

// motionCapable reports whether mode reports drags (mouse moved while a
// button is held) — only ModeMouseMotion (DECSET 1002) and ModeMouseMany
// (1003) do; plain button-click tracking (1000) and X10 (9) don't.
func motionCapable(mode emu.ModeFlag) bool {
	return mode&(emu.ModeMouseMotion|emu.ModeMouseMany) != 0
}

// EncodeButton encodes a mouse button press or release at 1-based
// terminal cell (col, row). ok is false if no tracking mode is active.
// Release events are suppressed (ok=false) under X10-only tracking
// (emu.ModeMouseX10 with none of Button/Motion/Many also set) — X10
// clients only ever expect press notifications, predating release
// tracking, and would misinterpret an unsolicited release report.
func EncodeButton(mode emu.ModeFlag, button Button, pressed bool, col, row int, mods Modifiers) (out []byte, ok bool) {
	if !TrackingEnabled(mode) {
		return nil, false
	}
	x10Only := mode&(emu.ModeMouseButton|emu.ModeMouseMotion|emu.ModeMouseMany) == 0
	if !pressed && x10Only {
		return nil, false
	}
	if mode&emu.ModeMouseSgr != 0 {
		return encodeButtonSGR(button, pressed, false, col, row, mods), true
	}
	return encodeButtonLegacy(button, pressed, false, col, row), true
}

// EncodeMotion encodes a drag (pointer moved while button is held) at
// 1-based terminal cell (col, row). ok is false unless mode has drag
// reporting enabled at all — see motionCapable.
func EncodeMotion(mode emu.ModeFlag, button Button, col, row int, mods Modifiers) (out []byte, ok bool) {
	if !motionCapable(mode) {
		return nil, false
	}
	if mode&emu.ModeMouseSgr != 0 {
		return encodeButtonSGR(button, true, true, col, row, mods), true
	}
	return encodeButtonLegacy(button, true, true, col, row), true
}

func encodeButtonSGR(button Button, pressed, motion bool, col, row int, mods Modifiers) []byte {
	cb := int(button) | modifierBits(mods)
	if motion {
		cb |= 32
	}
	suffix := byte('M')
	if !pressed {
		suffix = 'm'
	}
	return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, col, row, suffix))
}

func encodeButtonLegacy(button Button, pressed, motion bool, col, row int) []byte {
	var cb byte
	switch {
	case !pressed:
		cb = 3 // X10-style: release doesn't identify which button
	default:
		cb = byte(button)
	}
	if motion {
		cb |= 32
	}
	cx, cy := clampLegacyCoord(col), clampLegacyCoord(row)
	return []byte{0x1b, '[', 'M', cb + 32, cx + 32, cy + 32}
}

// clampLegacyCoord clamps a coordinate to what the legacy encoding's
// single-byte, +32-offset wire format can represent (max byte 255, so
// coordinate <= 223). Terminals needing larger coordinates should use SGR
// mode (1006) instead — there is no better fallback within this format.
func clampLegacyCoord(v int) byte {
	if v > 223 {
		return 223
	}
	if v < 0 {
		return 0
	}
	return byte(v)
}

func encodeSGR(dir Direction, col, row int, mods Modifiers) []byte {
	button := sgrWheelUp
	if dir == Down {
		button = sgrWheelDown
	}
	button |= modifierBits(mods)
	// SGR wheel events are always terminated with 'M' (there is no
	// press/release distinction for wheel ticks).
	return []byte(fmt.Sprintf("\x1b[<%d;%d;%dM", button, col, row))
}

func encodeLegacy(dir Direction, col, row int) []byte {
	cb := byte(legacyBase)
	if dir == Down {
		cb++
	}
	cx, cy := clampLegacyCoord(col), clampLegacyCoord(row)
	return []byte{0x1b, '[', 'M', cb, cx + 32, cy + 32}
}

func modifierBits(mods Modifiers) int {
	var b int
	if mods.contain(ModShift) {
		b |= 4
	}
	if mods.contain(ModAlt) {
		b |= 8
	}
	if mods.contain(ModCtrl) {
		b |= 16
	}
	return b
}
