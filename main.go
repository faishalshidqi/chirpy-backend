package main

import (
	"net/http"
)

func main() {
	serveMux := http.NewServeMux()
	var server = &http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	serveMux.Handle(
		"/app",
		http.StripPrefix("/app",
			http.FileServer(http.Dir("./")),
		),
	)
	serveMux.Handle(
		"/app/assets",
		http.StripPrefix("/app/assets",
			http.FileServer(http.Dir("./assets/")),
		),
	)
	serveMux.HandleFunc(
		"/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			w.Header().Set("Content-Type", "text/plain")
		},
	)

	server.ListenAndServe()
}
