package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/YoavIsaacs/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	database       *database.Queries
}

func (c *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (c *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
	} else {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		serverHits := c.fileserverHits.Load()
		output := fmt.Sprintf(
			`<html>
      <body>
        <h1>Welcome, Chirpy Admin</h1>
        <p>Chirpy has been visited %d times!</p>
      </body>
    </html>`,
			serverHits,
		)

		w.Write([]byte(output))
	}
}

func (c *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	isDev := os.Getenv("PLATFORM") == "dev"
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		fmt.Println(r.Method)
	} else if !isDev {
		w.WriteHeader(http.StatusForbidden)
	} else {
		c.fileserverHits.Store(0)
		c.database.ResetUsers(r.Context())
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("Hits reset to 0"))
	}
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
	} else {
		type parameters struct {
			Body string `json:"body"`
		}
		type returnValue struct {
			Error       string `json:"error"`
			CleanedBody string `json:"cleaned_body"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		responseBody := returnValue{}
		err := decoder.Decode(&params)
		if err != nil {
			responseBody.Error = "Something went wrong"
			w.WriteHeader(500)
		} else if len(params.Body) > 140 {
			responseBody.Error = "Chirp is too long"
			w.WriteHeader(400)
		} else {
			splitWords := strings.Fields(params.Body)
			for i, word := range splitWords {
				if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
					splitWords[i] = "****"
				}
			}
			responseBody.CleanedBody = strings.Join(splitWords, " ")
			w.WriteHeader(200)
		}
		dat, err := json.Marshal(responseBody)
		if err != nil {
			fmt.Printf("error: error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(dat)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
	} else {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}
}

func (c *apiConfig) addUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type userEmail struct {
		Email string `json:"email"`
	}

	type responseLower struct {
		ID         uuid.UUID `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Email      string    `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	emailDecoded := userEmail{}
	err := decoder.Decode(&emailDecoded)
	if err != nil {
		fmt.Printf("error: error decoding json: %s", err)
		w.WriteHeader(500)
		return
	}
	email := emailDecoded.Email
	createdUsr, err := c.database.CreateUser(ctx, email)
	if err != nil {
		fmt.Printf("error: error creating new user: %s", err)
		w.WriteHeader(500)
		return
	}

	userResp := responseLower{
		ID:         createdUsr.ID,
		Created_at: createdUsr.CreatedAt,
		Updated_at: createdUsr.UpdatedAt,
		Email:      createdUsr.Email,
	}

	responseData, err := json.Marshal(userResp)
	if err != nil {
		fmt.Printf("error: error decoding response: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(responseData)
}

func main() {
	mux := http.NewServeMux()
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Println("error: error loading .env file")
		return
	}

	cfg := &apiConfig{}

	fileServer := http.FileServer(http.Dir("."))

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("error: server error")
		return
	}

	dbQueries := database.New(db)
	cfg.database = dbQueries

	mux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))
	mux.HandleFunc("GET /api/healthz", healthCheckHandler)
	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	mux.HandleFunc("POST /api/users", cfg.addUserHandler)
	serv := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	err = serv.ListenAndServe()
	if err != nil {
		fmt.Println("error: server error")
	}
}
