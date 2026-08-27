package app

import (
	"image/color"

	"golang.org/x/image/font"

	"github.com/geckty/geckty/internal/ui/input"
	"github.com/geckty/geckty/internal/ui/termview"
)

// Compatibility aliases for the former monolithic gogpu UI package.
// Prefer importing input/termview directly in new code.

type (
	// Action is an alias for input.Action.
	Action = input.Action
	// Keymap is an alias for input.Keymap.
	Keymap = input.Keymap
	// Painter is an alias for termview.Painter.
	Painter = termview.Painter
	// Selection is an alias for termview.Selection.
	Selection = termview.Selection
	// Placement is an alias for termview.Placement.
	Placement = termview.Placement
	fontBundle = termview.FontBundle
)

// Action name constants re-exported from input for local switch/dispatch.
const (
	ActionNewTab              = input.ActionNewTab
	ActionCloseTab            = input.ActionCloseTab
	ActionNextTab             = input.ActionNextTab
	ActionPrevTab             = input.ActionPrevTab
	ActionCopy                = input.ActionCopy
	ActionPaste               = input.ActionPaste
	ActionSearchScrollback    = input.ActionSearchScrollback
	ActionOpenURLHints        = input.ActionOpenURLHints
	ActionShowScrollback      = input.ActionShowScrollback
	ActionIncreaseFontSize    = input.ActionIncreaseFontSize
	ActionDecreaseFontSize    = input.ActionDecreaseFontSize
	ActionResetFontSize       = input.ActionResetFontSize
	ActionSplitVertical       = input.ActionSplitVertical
	ActionSplitHorizontal     = input.ActionSplitHorizontal
	ActionNextPane            = input.ActionNextPane
	ActionPrevPane            = input.ActionPrevPane
	ActionClosePane           = input.ActionClosePane
	ActionScrollToPrevPrompt  = input.ActionScrollToPrevPrompt
	ActionScrollToNextPrompt  = input.ActionScrollToNextPrompt
	ActionSelectLastCmdOutput = input.ActionSelectLastCmdOutput

	roleMono = termview.RoleMono
	roleUI   = termview.RoleUI
)

// Constructors / encoders re-exported from input.
var (
	// NewKeymap builds a Keymap from config keybindings.
	NewKeymap = input.NewKeymap
	// EncodeKey encodes a key press for the PTY.
	EncodeKey = input.EncodeKey
	// EncodeText encodes pasted/typed text for the PTY.
	EncodeText = input.EncodeText
)

const osWindows = "windows"

func loadFontBundle(family string, size, scale float64, role termview.FontRole) termview.FontBundle {
	return termview.LoadFontBundle(family, size, scale, role)
}

func loadSymbolFallbackFace(size, scale float64) font.Face {
	return termview.LoadSymbolFallbackFace(size, scale)
}

func toRGBA(c color.NRGBA) color.RGBA { return termview.ToRGBA(c) }

func withAlpha(c color.RGBA, a uint8) color.RGBA { return termview.WithAlpha(c, a) }
