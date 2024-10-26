package main

import (
	"bootdevHTTPServer/internal/auth"
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
	jwtSecret      []byte
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
	jwtSecret := os.Getenv("JWT_SECRET")

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		_ = fmt.Errorf("error opening database connection: %v", err)
	}

	serveMux := http.NewServeMux()
	config := apiConfig{
		fileServerHits: atomic.Int32{},
		dbQueries:      database.New(db),
		jwtSecret:      []byte(jwtSecret),
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
			err := config.dbQueries.EmptyUsersTable(r.Context())
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
		"/api/chirps",
		func(w http.ResponseWriter, r *http.Request) {
			type errorAndMessage struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if r.Method == "POST" {
				type parameters struct {
					Body   string    `json:"body"`
					UserID uuid.UUID `json:"user_id"`
				}

				type response struct {
					ID        uuid.UUID `json:"id"`
					CreatedAt time.Time `json:"created_at"`
					UpdatedAt time.Time `json:"updated_at"`
					Body      string    `json:"body"`
					UserID    uuid.UUID `json:"user_id"`
				}
				decoder := json.NewDecoder(r.Body)
				params := parameters{}
				err := decoder.Decode(&params)
				if err != nil {
					dat, err := json.Marshal(errorAndMessage{
						Error: "error marshalling JSON: " + err.Error(),
					})
					if err != nil {
						log.Printf("error writing /api/chirps response: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusBadRequest)
					w.Write(dat)
					return
				}

				if len(params.Body) > 140 {
					dat, err := json.Marshal(errorAndMessage{
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
					bearerToken, bearerTokenErr := auth.GetBearerToken(r.Header)
					if bearerTokenErr != nil {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					userID, err := auth.ValidateJWT(bearerToken, string(config.jwtSecret))
					if err != nil {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					chirp, err := config.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
						Body:   strings.Join(body, " "),
						UserID: userID,
					})
					if err != nil {
						return
					}
					dat, err := json.Marshal(response{
						ID:        chirp.ID,
						CreatedAt: chirp.CreatedAt,
						UpdatedAt: chirp.UpdatedAt,
						Body:      chirp.Body,
						UserID:    chirp.UserID,
					})
					if err != nil {
						log.Printf("error writing /validate_chirp response: %v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusCreated)
					w.Write(dat)
					return

				}
			} else if r.Method == "GET" {
				type response struct {
					ID        uuid.UUID `json:"id"`
					CreatedAt time.Time `json:"created_at"`
					UpdatedAt time.Time `json:"updated_at"`
					Body      string    `json:"body"`
					UserID    uuid.UUID `json:"user_id"`
				}
				if r.Method != "GET" {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Add("Content-Type", "application/json")
				chirps, err := config.dbQueries.RetrieveChirps(r.Context())
				if err != nil {
					return
				}
				retChirps := make([]response, len(chirps))
				for i, chirp := range chirps {
					retChirps[i] = response{
						ID:        chirp.ID,
						CreatedAt: chirp.CreatedAt,
						UpdatedAt: chirp.UpdatedAt,
						Body:      chirp.Body,
						UserID:    chirp.UserID,
					}
				}
				dat, err := json.Marshal(retChirps)
				w.Write(dat)
			}
		},
	)
	go serveMux.HandleFunc(
		"/api/chirps/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			type errorAndMessage struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			type response struct {
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Body      string    `json:"body"`
				UserID    uuid.UUID `json:"user_id"`
			}
			id := r.PathValue("id")
			if id != "" {
				chirp, err := config.dbQueries.RetrieveChirpById(r.Context(), uuid.MustParse(id))
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				if chirp.Body == "" {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				dat, err := json.Marshal(response{
					ID:        chirp.ID,
					CreatedAt: chirp.CreatedAt,
					UpdatedAt: chirp.UpdatedAt,
					Body:      chirp.Body,
					UserID:    chirp.UserID,
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
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			type errorAndMessage struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			type response struct {
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
			hashed, err := auth.HashPassword(params.Password)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			user, createUserErr := config.dbQueries.CreateUser(
				r.Context(),
				database.CreateUserParams{
					Email:          email.Address,
					HashedPassword: hashed,
				},
			)
			if createUserErr != nil {
				return
			}
			if err != nil {
				dat, err := json.Marshal(errorAndMessage{
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

	go serveMux.HandleFunc(
		"/api/login",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			type parameters struct {
				Email            string `json:"email"`
				Password         string `json:"password"`
				ExpiresInSeconds int    `json:"expires_in_seconds"`
			}
			type errorAndMessage struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			type response struct {
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Email     string    `json:"email"`
				Token     string    `json:"token"`
			}
			w.Header().Add("Content-Type", "application/json")
			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			user, dbErr := config.dbQueries.GetUserByEmail(r.Context(), params.Email)
			if dbErr != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			hashErr := auth.CheckPasswordHash(params.Password, user.HashedPassword)
			if hashErr != nil {
				marshal, _ := json.Marshal(errorAndMessage{
					Message: "Incorrect email or password",
				})
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(marshal)
				return
			}
			var expiresInSeconds int
			if params.ExpiresInSeconds == 0 || params.ExpiresInSeconds > 3600 {
				expiresInSeconds = 3600
			} else {
				expiresInSeconds = params.ExpiresInSeconds
			}
			jwt, JWTerr := auth.MakeJWT(user.ID, string(config.jwtSecret), time.Duration(expiresInSeconds))
			if JWTerr != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			data, err := json.Marshal(response{
				ID:        user.ID,
				Email:     user.Email,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
				Token:     jwt,
			})
			if err != nil {
				return
			}
			w.Write(data)
		},
	)

	err = server.ListenAndServe()
	if err != nil {
		_ = fmt.Errorf("server isn't starting")
	}
}
