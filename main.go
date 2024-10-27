package main

import (
	"chirpy/internal/auth"
	"chirpy/internal/database"
	"chirpy/internal/utils"
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
	config := utils.ApiConfig{
		FileServerHits: atomic.Int32{},
		DbQueries:      database.New(db),
		JwtSecret:      []byte(jwtSecret),
	}
	var server = &http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	go serveMux.Handle(
		"/app",
		http.StripPrefix("/app",
			config.MiddlewareMetricsInc(
				http.FileServer(http.Dir("./")),
			),
		),
	)
	go serveMux.Handle(
		"/app/assets/",
		http.StripPrefix("/app/assets",
			config.MiddlewareMetricsInc(
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
			err := config.DbQueries.EmptyUsersTable(r.Context())
			if err != nil {
				return
			}
			config.FileServerHits.Store(0)

		},
	)
	go serveMux.HandleFunc(
		"/admin/metrics",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			num := config.FileServerHits.Load()
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
					dat, err := json.Marshal(utils.Error{
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
					dat, err := json.Marshal(utils.Message{
						Message: "Chirp is too long",
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
					userID, err := auth.ValidateJWT(bearerToken, string(config.JwtSecret))
					if err != nil {
						marshal, _ := json.Marshal(utils.Error{
							Error: err.Error(),
						})
						w.WriteHeader(http.StatusUnauthorized)
						w.Write(marshal)
						return
					}
					chirp, err := config.DbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
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
				chirps, err := config.DbQueries.RetrieveChirps(r.Context())
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
			type response struct {
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Body      string    `json:"body"`
				UserID    uuid.UUID `json:"user_id"`
			}
			id := r.PathValue("id")
			if id != "" {
				chirp, err := config.DbQueries.RetrieveChirpById(r.Context(), uuid.MustParse(id))
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
			type response struct {
				ID        uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Email     string    `json:"email"`
			}
			if r.Method == "POST" {
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
				user, createUserErr := config.DbQueries.CreateUser(
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
					dat, err := json.Marshal(utils.Error{
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
			} else if r.Method == "PUT" {
				type parameters struct {
					Password string `json:"password"`
					Email    string `json:"email"`
				}
				type response struct {
					ID        uuid.UUID `json:"id"`
					CreatedAt time.Time `json:"created_at"`
					UpdatedAt time.Time `json:"updated_at"`
					Email     string    `json:"email"`
				}
				bearerToken, err := auth.GetBearerToken(r.Header)
				if err != nil {
					marshal, _ := json.Marshal(utils.Error{
						Error: "invalid authentication token",
					})
					w.WriteHeader(http.StatusUnauthorized)
					w.Write(marshal)
					return
				}
				userID, validateErr := auth.ValidateJWT(bearerToken, string(config.JwtSecret))
				if validateErr != nil {
					marshal, _ := json.Marshal(utils.Error{
						Error: "invalid authentication token",
					})
					w.WriteHeader(http.StatusUnauthorized)
					w.Write(marshal)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				decoder := json.NewDecoder(r.Body)
				params := parameters{}
				decodeErr := decoder.Decode(&params)
				if decodeErr != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				email, emailParseErr := mail.ParseAddress(params.Email)
				if emailParseErr != nil {
					marshal, _ := json.Marshal(utils.Error{
						Error: "invalid email",
					})
					w.WriteHeader(http.StatusBadRequest)
					w.Write(marshal)
					return
				}
				user, getUserErr := config.DbQueries.GetUserById(r.Context(), userID)
				if getUserErr != nil {
					log.Printf("error getting user by email %v: %v", params.Email, getUserErr)
					marshal, _ := json.Marshal(utils.Error{
						Error: fmt.Sprintf("email %v not found", email.Address),
					})
					w.WriteHeader(http.StatusNotFound)
					w.Write(marshal)
					return
				}
				hashedPassword, _ := auth.HashPassword(params.Password)
				updatedUser, updateUserErr := config.DbQueries.UpdateUserByID(
					r.Context(),
					database.UpdateUserByIDParams{
						ID:             userID,
						Email:          email.Address,
						CreatedAt:      user.CreatedAt,
						HashedPassword: hashedPassword,
					},
				)
				if updateUserErr != nil {
					log.Printf("error updating user: %v", updateUserErr)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				dat, err := json.Marshal(response{
					ID:        updatedUser.ID,
					CreatedAt: updatedUser.CreatedAt,
					UpdatedAt: updatedUser.UpdatedAt,
					Email:     updatedUser.Email,
				})
				if err != nil {
					log.Printf("error writing PUT /api/users response: %v", err)
					return
				}
				w.Write(dat)
				return
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
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
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			type response struct {
				ID           uuid.UUID `json:"id"`
				CreatedAt    time.Time `json:"created_at"`
				UpdatedAt    time.Time `json:"updated_at"`
				Email        string    `json:"email"`
				Token        string    `json:"token"`
				RefreshToken string    `json:"refresh_token"`
			}
			w.Header().Add("Content-Type", "application/json")
			decoder := json.NewDecoder(r.Body)
			params := parameters{}
			err := decoder.Decode(&params)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			user, dbErr := config.DbQueries.GetUserByEmail(r.Context(), params.Email)
			if dbErr != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			hashErr := auth.CheckPasswordHash(params.Password, user.HashedPassword)
			if hashErr != nil {
				marshal, _ := json.Marshal(utils.Message{
					Message: "Incorrect email or password",
				})
				w.WriteHeader(http.StatusUnauthorized)
				w.Write(marshal)
				return
			}
			accessToken, JWTerr := auth.MakeJWT(user.ID, string(config.JwtSecret), time.Hour)
			if JWTerr != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			refreshToken, err := auth.MakeRefreshToken()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = config.DbQueries.CreateRefreshToken(
				r.Context(),
				database.CreateRefreshTokenParams{
					Token:     refreshToken,
					UserID:    user.ID,
					ExpiresAt: time.Now().Add(24 * 60 * time.Hour),
				},
			)
			if err != nil {
				return
			}
			data, err := json.Marshal(response{
				ID:           user.ID,
				Email:        user.Email,
				CreatedAt:    user.CreatedAt,
				UpdatedAt:    user.UpdatedAt,
				Token:        accessToken,
				RefreshToken: refreshToken,
			})
			if err != nil {
				return
			}
			w.Write(data)
		},
	)
	go serveMux.HandleFunc(
		"/api/refresh",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			type response struct {
				Token string `json:"token"`
			}
			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			refreshToken, err := config.DbQueries.GetRefreshTokenByToken(
				r.Context(),
				bearerToken,
			)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if refreshToken.RevokedAt.Valid {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if refreshToken.ExpiresAt.Before(time.Now()) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			accessToken, err := auth.MakeJWT(refreshToken.UserID, string(config.JwtSecret), time.Hour)
			if err != nil {
				return
			}
			marshal, err := json.Marshal(response{
				Token: accessToken,
			})
			if err != nil {
				return
			}
			w.Write(marshal)
			return
		},
	)

	go serveMux.HandleFunc(
		"/api/revoke",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			bearerToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			refreshToken, err := config.DbQueries.GetRefreshTokenByToken(r.Context(), bearerToken)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			err = config.DbQueries.UpdateRefreshTokenByToken(
				r.Context(),
				database.UpdateRefreshTokenByTokenParams{
					Token:     refreshToken.Token,
					CreatedAt: refreshToken.CreatedAt,
					UserID:    refreshToken.UserID,
					ExpiresAt: time.Now().Add(24 * 60 * time.Hour),
					RevokedAt: sql.NullTime{Time: time.Now(), Valid: true},
				},
			)
			if err != nil {
				return
			}
			w.WriteHeader(http.StatusNoContent)
		},
	)

	err = server.ListenAndServe()
	if err != nil {
		_ = fmt.Errorf("server isn't starting")
	}
}
