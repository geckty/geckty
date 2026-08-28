package plugin

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// PermissionLog allows host log().
	PermissionLog = "log"
	// PermissionStatusbar allows statusbar_draw().
	PermissionStatusbar = "statusbar"
)

// PermissionSet is a plugin's declared capability set (manifest permissions).
type PermissionSet map[string]bool

// Has reports whether name was granted.
func (p PermissionSet) Has(name string) bool {
	return p[name]
}

// knownPermissions is the fixed set of capability strings a plugin.toml's
// permissions array may contain — deny-by-default, like
// internal/ui/input.Keymap rejects unknown keybinding actions: an
// unrecognized permission (a typo, or a capability a future geckty version
// hasn't added yet) fails to load rather than being silently granted
// nothing or silently ignored.
var knownPermissions = PermissionSet{
	PermissionLog:       true,
	PermissionStatusbar: true,
}

// Manifest is a parsed plugin.toml: name, version, and entry are metadata;
// permissions is the plugin's declared capability set, checked against
// knownPermissions and enforced per-call by the host functions in api.go.
type Manifest struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Entry       string   `toml:"entry"`
	Permissions []string `toml:"permissions"`
}

// loadManifest reads and validates dir/plugin.toml.
func loadManifest(dir string) (Manifest, error) {
	var m Manifest
	path := filepath.Join(dir, "plugin.toml")
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return Manifest{}, fmt.Errorf("plugin manifest %s: %w", path, err)
	}
	if m.Name == "" {
		return Manifest{}, fmt.Errorf("plugin manifest %s: missing required \"name\"", path)
	}
	if m.Entry == "" {
		return Manifest{}, fmt.Errorf("plugin manifest %s: missing required \"entry\"", path)
	}
	for _, p := range m.Permissions {
		if !knownPermissions[p] {
			return Manifest{}, fmt.Errorf("plugin manifest %s: unknown permission %q", path, p)
		}
	}
	return m, nil
}
