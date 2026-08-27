package config

// Keybinding action names — the single source of truth for default binding
// strings and for internal/ui/input.Action values. Keep in sync with the
// keymap validator: adding an action here without registering it in
// input.validActions will reject the binding at load time.
const (
	ActionNewTab              = "new_tab"
	ActionCloseTab            = "close_tab"
	ActionNextTab             = "next_tab"
	ActionPrevTab             = "prev_tab"
	ActionCopy                = "copy"
	ActionPaste               = "paste"
	ActionSearchScrollback    = "search_scrollback"
	ActionOpenURLHints        = "open_url_hints"
	ActionShowScrollback      = "show_scrollback"
	ActionIncreaseFontSize    = "increase_font_size"
	ActionDecreaseFontSize    = "decrease_font_size"
	ActionResetFontSize       = "reset_font_size"
	ActionSplitVertical       = "split_vertical"
	ActionSplitHorizontal     = "split_horizontal"
	ActionNextPane            = "next_pane"
	ActionPrevPane            = "prev_pane"
	ActionClosePane           = "close_pane"
	ActionScrollToPrevPrompt  = "scroll_to_prev_prompt"
	ActionScrollToNextPrompt  = "scroll_to_next_prompt"
	ActionSelectLastCmdOutput = "select_last_command_output"
)

// Modifier names accepted in [[keybindings]] mods arrays.
const (
	ModCtrl  = "ctrl"
	ModShift = "shift"
	ModAlt   = "alt"
	ModCmd   = "cmd"
)
