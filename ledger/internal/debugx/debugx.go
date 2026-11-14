package debugx

import (
	"expvar"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sync"
	"time"
)

var publishOnce sync.Once

func NewServer(addr string) *http.Server {
	publishOnce.Do(func() {
		expvar.Publish("goroutines", expvar.Func(func() any {
			return runtime.NumGoroutine()
		}))
	})

	mux := http.NewServeMux()
	mux.Handle("GET /debug/vars", expvar.Handler())

	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
