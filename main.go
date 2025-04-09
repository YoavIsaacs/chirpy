package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func (c *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type createdChirpLower struct {
		ID         uuid.UUID `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Body       string    `json:"body"`
		User_id    uuid.UUID `json:"user_id"`
	}

	retValue := []createdChirpLower{}

	resp, err := c.database.GetAllChirps(r.Context())
	if err != nil {
		fmt.Printf("error: error getting all chirps: %s", err)
		w.WriteHeader(500)
		return
	}

	temp := createdChirpLower{}
	for _, chirp := range resp {
		temp.ID = chirp.ID
		temp.Created_at = chirp.CreatedAt
		temp.Updated_at = chirp.UpdatedAt
		temp.Body = chirp.Body
		temp.User_id = chirp.UserID

		retValue = append(retValue, temp)
	}
	responseData, err := json.Marshal(retValue)
	if err != nil {
		fmt.Printf("error: error marshalling chirps: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

func (c *apiConfig) addChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type inputPayload struct {
		Body    string    `json:"body"`
		User_id uuid.UUID `json:"user_id"`
	}

	type createdChirpLower struct {
		ID         uuid.UUID `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Body       string    `json:"body"`
		User_id    uuid.UUID `json:"user_id"`
	}

	type queryParams struct {
		body    string
		user_id uuid.UUID
	}

	decoder := json.NewDecoder(r.Body)
	payload := inputPayload{}
	err := decoder.Decode(&payload)
	if err != nil {
		fmt.Printf("error: error decoding json: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(payload.Body) > 140 {
		fmt.Println("error: chirp body length exceeds 140 characters")
		w.WriteHeader(400)
		return
	}

	if len(payload.Body) == 0 {
		fmt.Println("error: chirp body cannot be empty")
		w.WriteHeader(400)
		return
	}

	params := database.CreateChirpParams{
		Body:   payload.Body,
		UserID: payload.User_id,
	}

	createdChirp, err := c.database.CreateChirp(r.Context(), params)
	if err != nil {
		fmt.Printf("error: error creating new chirp: %s", err)
		w.WriteHeader(500)
		return
	}
	newChirpDataLower := createdChirpLower{
		ID:         createdChirp.ID,
		Created_at: createdChirp.CreatedAt,
		Updated_at: createdChirp.UpdatedAt,
		Body:       createdChirp.Body,
		User_id:    createdChirp.UserID,
	}
	responseData, err := json.Marshal(newChirpDataLower)
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
	mux.HandleFunc("POST /api/users", cfg.addUserHandler)
	mux.HandleFunc("POST /api/chirps", cfg.addChirpsHandler)
	mux.HandleFunc("GET /api/chirps", cfg.getAllChirpsHandler)
	serv := http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	err = serv.ListenAndServe()
	if err != nil {
		fmt.Println("error: server error")
	}
}
