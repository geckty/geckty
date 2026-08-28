// Package debug holds opt-in development helpers (profiling, diagnostics).
package debug

import (
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"
)

// MaybeStartPprof listens when GECKTY_PPROF is set (e.g. ":6060" or "localhost:6060").
func MaybeStartPprof() {
	addr := os.Getenv("GECKTY_PPROF")
	if addr == "" {
		return
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort("127.0.0.1", addr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           mux,
	}
	go func() {
		log.Printf("geckty: pprof listening on http://%s/debug/pprof/", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("geckty: pprof server: %v", err)
		}
	}()
}
