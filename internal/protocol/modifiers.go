package protocol

// Modifiers is a toolkit-agnostic set of keyboard modifiers. Protocol
// packages do not import gpucontext — UI callers translate at the boundary.
type Modifiers uint8

// Modifier bits (matching Kitty / SGR mouse encoding order for the first three).
const (
	ModShift Modifiers = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
)

// Contain reports whether all bits in o are set in m.
func (m Modifiers) Contain(o Modifiers) bool { return m&o == o }
