package config

import (
	"os"
	"time"
)

// watchPollInterval is how often Watch stats the config file. A plain stat
// poll (rather than a filesystem-notify API) keeps this dependency-free and
// behaves the same across every OS and editor save style (including
// write-to-temp-then-rename, which preserves the watched path); a config
// file is small and read rarely enough that the poll cost is negligible.
const watchPollInterval = 500 * time.Millisecond

// Watch polls the file c was loaded from (see Load) and calls onChange with
// a freshly parsed Config each time its mtime advances. Parse errors (e.g.
// the file mid-save, or a typo) are passed as (nil, err) so the caller can
// log and keep running on the last-known-good config rather than crash.
// Returns a stop func that ends the poll goroutine; safe to call once.
//
// A Config not obtained from Load (e.g. Default() directly) has no source
// path, so Watch is a no-op returning an immediate no-op stop func.
func (c *Config) Watch(onChange func(*Config, error)) (stop func()) {
	if c.sourcePath == "" {
		return func() {}
	}
	path := c.sourcePath

	var lastMod time.Time
	if fi, err := os.Stat(path); err == nil {
		lastMod = fi.ModTime()
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(watchPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fi, err := os.Stat(path)
				if err != nil || !fi.ModTime().After(lastMod) {
					continue
				}
				lastMod = fi.ModTime()
				onChange(Load(path))
			}
		}
	}()
	return func() { close(done) }
}
