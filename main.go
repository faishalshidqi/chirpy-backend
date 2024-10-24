package main

import (
	"bootdevHTTPServer/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries      *database.Queries
}

func (config *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return
	}
	dbUrl := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		_ = fmt.Errorf("error opening database connection: %v", err)
	}

	serveMux := http.NewServeMux()
	config := apiConfig{
		fileServerHits: atomic.Int32{},
		dbQueries:      database.New(db),
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
		"/app/assets/",
		http.StripPrefix("/app/assets",
			config.middlewareMetricsInc(
				http.FileServer(http.Dir("./assets/")),
			),
		),
	)
	go serveMux.HandleFunc(
		"/api/healthz",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			_, err := w.Write([]byte("OK"))
			if err != nil {
				_ = fmt.Errorf("error writing /healthz response: %v", err)
			}
			w.Header().Set("Content-Type", "text/plain")
		},
	)
	go serveMux.HandleFunc(
		"/admin/reset",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if platform != "dev" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, err := config.dbQueries.EmptyUsersTable(r.Context())
			if err != nil {
				return
			}
			config.fileServerHits.Store(0)

		},
	)
	go serveMux.HandleFunc(
		"/admin/metrics",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			num := config.fileServerHits.Load()
			msg := fmt.Sprintf("<html>\n  <body>\n    <h1>Welcome, Chirpy Admin</h1>\n    <p>Chirpy has been visited %d times!</p>\n  </body>\n</html>", num)
			w.Header().Set("Content-Type", "text/html")
			_, err := w.Write([]byte(msg))
			if err != nil {
				_ = fmt.Errorf("error writing /metrics response: %v", err)
			}
		},
	)
	go serveMux.HandleFunc(
		"/api/validate_chirp",
		func(w http.ResponseWriter, r *http.Request) {
			type parameters struct {
				Body string `json:"body"`
			}

			type response struct {
				Valid       bool   `json:"valid"`
				Error       string `json:"error"`
				CleanedBody string `json:"cleaned_body"`
			}

			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				dat, err := json.Marshal(response{
					Error: "error marshalling JSON: " + err.Error(),
				})
				if err != nil {
					log.Printf("error writing /validate_chirp response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				w.Write(dat)
				return
			}

			if len(params.Body) > 140 {
				dat, err := json.Marshal(response{
					Error: "Chirp is too long",
				})
				if err != nil {
					log.Printf("error writing /validate_chirp response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				w.Write(dat)
				return
			} else {
				w.Header().Add("Content-Type", "application/json")
				body := strings.Split(params.Body, " ")
				strings.ReplaceAll(body[0], " ", "")
				for i, word := range body {
					if strings.ToLower(word) == "kerfuffle" {
						body[i] = "****"
					}
					if strings.ToLower(word) == "sharbert" {
						body[i] = "****"
					}
					if strings.ToLower(word) == "fornax" {
						body[i] = "****"
					}
				}
				dat, err := json.Marshal(response{
					Valid:       true,
					CleanedBody: strings.Join(body, " "),
				})
				if err != nil {
					log.Printf("error writing /validate_chirp response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.Write(dat)
				return

			}

		},
	)
	go serveMux.HandleFunc(
		"/api/users",
		func(w http.ResponseWriter, r *http.Request) {
			type parameters struct {
				Email string `json:"email"`
			}
			type response struct {
				Error     string    `json:"error"`
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Email     string    `json:"email"`
			}
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Add("Content-Type", "application/json")
			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			email, emailParseErr := mail.ParseAddress(params.Email)
			if emailParseErr != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			user, createUserErr := config.dbQueries.CreateUser(r.Context(), email.Address)
			if createUserErr != nil {
				return
			}
			if err != nil {
				dat, err := json.Marshal(response{
					Error: "error marshalling JSON: " + err.Error(),
				})
				if err != nil {
					log.Printf("error writing /api/users response: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				w.Write(dat)
				return
			}
			dat, err := json.Marshal(response{
				ID:        user.ID,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
				Email:     user.Email,
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write(dat)
		},
	)

	err = server.ListenAndServe()
	if err != nil {
		_ = fmt.Errorf("server isn't starting")
	}
}
