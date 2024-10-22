package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (config *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {
	serveMux := http.NewServeMux()
	config := apiConfig{
		fileServerHits: atomic.Int32{},
	}
	var server = &http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	go serveMux.Handle(
		"/app",
		http.StripPrefix("/app",
			config.middlewareMetricsInc(
				http.FileServer(http.Dir("./")),
			),
		),
	)
	go serveMux.Handle(
		"/app/assets",
		http.StripPrefix("/app/assets",
			http.FileServer(http.Dir("./assets/")),
		),
	)
	go serveMux.HandleFunc(
		"/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte("OK"))
			if err != nil {
				_ = fmt.Errorf("error writing /healthz response: %v", err)
			}
			w.Header().Set("Content-Type", "text/plain")
		},
	)
	go serveMux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		config.fileServerHits.Store(0)
	})
	go serveMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		num := config.fileServerHits.Load()
		msg := fmt.Sprintf("Hits: %v", num)
		_, err := w.Write([]byte(msg))
		if err != nil {
			_ = fmt.Errorf("error writing /metrics response: %v", err)
		}
	})

	err := server.ListenAndServe()
	if err != nil {
		_ = fmt.Errorf("server isn't starting")
	}
}
