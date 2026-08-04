package gogpu

import (
	"github.com/gogpu/gpucontext"
)

var namedKeys = map[gpucontext.Key][]byte{
	gpucontext.KeyEnter:       {'\r'},
	gpucontext.KeyNumpadEnter: {'\r'},
	gpucontext.KeyBackspace:   {0x7f},
	gpucontext.KeyDelete:      {0x1b, '[', '3', '~'},
	gpucontext.KeyTab:         {'\t'},
	gpucontext.KeyEscape:      {0x1b},
	gpucontext.KeyLeft:        {0x1b, '[', 'D'},
	gpucontext.KeyRight:       {0x1b, '[', 'C'},
	gpucontext.KeyUp:          {0x1b, '[', 'A'},
	gpucontext.KeyDown:        {0x1b, '[', 'B'},
	gpucontext.KeyHome:        {0x1b, '[', 'H'},
	gpucontext.KeyEnd:         {0x1b, '[', 'F'},
	gpucontext.KeyPageUp:      {0x1b, '[', '5', '~'},
	gpucontext.KeyPageDown:    {0x1b, '[', '6', '~'},
	gpucontext.KeyF1:          {0x1b, 'O', 'P'},
	gpucontext.KeyF2:          {0x1b, 'O', 'Q'},
	gpucontext.KeyF3:          {0x1b, 'O', 'R'},
	gpucontext.KeyF4:          {0x1b, 'O', 'S'},
	gpucontext.KeyF5:          {0x1b, '[', '1', '5', '~'},
	gpucontext.KeyF6:          {0x1b, '[', '1', '7', '~'},
	gpucontext.KeyF7:          {0x1b, '[', '1', '8', '~'},
	gpucontext.KeyF8:          {0x1b, '[', '1', '9', '~'},
	gpucontext.KeyF9:          {0x1b, '[', '2', '0', '~'},
	gpucontext.KeyF10:         {0x1b, '[', '2', '1', '~'},
	gpucontext.KeyF11:         {0x1b, '[', '2', '3', '~'},
	gpucontext.KeyF12:         {0x1b, '[', '2', '4', '~'},
}

// ctrlLetterKeys maps gpucontext's physical A-Z key codes to their
// alphabet position (1-26) for Ctrl+letter control-byte encoding.
var ctrlLetterKeys = map[gpucontext.Key]byte{
	gpucontext.KeyA: 1, gpucontext.KeyB: 2, gpucontext.KeyC: 3, gpucontext.KeyD: 4,
	gpucontext.KeyE: 5, gpucontext.KeyF: 6, gpucontext.KeyG: 7, gpucontext.KeyH: 8,
	gpucontext.KeyI: 9, gpucontext.KeyJ: 10, gpucontext.KeyK: 11, gpucontext.KeyL: 12,
	gpucontext.KeyM: 13, gpucontext.KeyN: 14, gpucontext.KeyO: 15, gpucontext.KeyP: 16,
	gpucontext.KeyQ: 17, gpucontext.KeyR: 18, gpucontext.KeyS: 19, gpucontext.KeyT: 20,
	gpucontext.KeyU: 21, gpucontext.KeyV: 22, gpucontext.KeyW: 23, gpucontext.KeyX: 24,
	gpucontext.KeyY: 25, gpucontext.KeyZ: 26,
}

// EncodeKey converts a key-press into the bytes to write to the shell. ok
// is false for keys Encode has nothing to say about (e.g. a plain letter/
// digit whose text arrives via the SetOnTextInput callback instead).
//
// gpucontext.Key is a platform-independent, physical/positional key code
// (USB-HID-usage-code-like) delivered via a separate SetOnKeyPress
// callback — text arrives via a distinct SetOnTextInput callback.
func EncodeKey(k gpucontext.Key, mods gpucontext.Modifiers) (out []byte, ok bool) {
	if mods.HasControl() {
		if n, ok := ctrlLetterKeys[k]; ok {
			return []byte{n}, true
		}
	}
	if b, ok := namedKeys[k]; ok {
		return b, true
	}
	return nil, false
}

// EncodeText returns the bytes for a SetOnTextInput callback's literal text.
func EncodeText(text string) []byte {
	return []byte(text)
}
