package main

import (
	"net/http"
)

func main() {
	serveMux := http.NewServeMux()
	var server = http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	fs := http.FileServer(http.Dir("."))
	serveMux.Handle(
		"/",
		fs,
	)

	server.ListenAndServe()
}
