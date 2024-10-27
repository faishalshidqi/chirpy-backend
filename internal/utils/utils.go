package utils

import (
	"chirpy/internal/database"
	"net/http"
	"sync/atomic"
)

type ApiConfig struct {
	FileServerHits atomic.Int32
	DbQueries      *database.Queries
	JwtSecret      []byte
	PolkaKey       string
}

func (config *ApiConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config.FileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

type Error struct {
	Error string `json:"error"`
}

type Message struct {
	Message string `json:"message"`
}
