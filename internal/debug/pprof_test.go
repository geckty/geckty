package debug

import "testing"

func TestMaybeStartPprofUnsetIsNoop(t *testing.T) {
	t.Setenv("GECKTY_PPROF", "")
	MaybeStartPprof()
}

func TestMaybeStartPprofBarePort(t *testing.T) {
	t.Setenv("GECKTY_PPROF", "0")
	MaybeStartPprof()
}

func TestMaybeStartPprofHostPort(t *testing.T) {
	t.Setenv("GECKTY_PPROF", "127.0.0.1:0")
	MaybeStartPprof()
}
