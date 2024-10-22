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
		"/",
		http.FileServer(http.Dir(".")),
	)
	serveMux.Handle(
		"/assets",
		http.FileServer(http.Dir("./assets/")),
	)

	server.ListenAndServe()
}
