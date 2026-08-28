package debug

import (
	"log"
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof/*
	"os"
)

// MaybeStartPprof listens when GECKTY_PPROF is set (e.g. ":6060" or "localhost:6060").
func MaybeStartPprof() {
	addr := os.Getenv("GECKTY_PPROF")
	if addr == "" {
		return
	}
	go func() {
		log.Printf("geckty: pprof listening on http://%s/debug/pprof/", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("geckty: pprof server: %v", err)
		}
	}()
}
