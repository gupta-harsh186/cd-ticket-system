package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var jwtSecret string

// Models
type User struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Password string `json:"-"`
}

type Ticket struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	Email string `json:"email"`
}

type TicketRequest struct {
	Title string `json:"title"`
}

type StatusUpdateRequest struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

// Claims for JWT
type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func init() {
	jwtSecret = os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production"
	}
}

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./tickets.db")
	if err != nil {
		return err
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tickets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		status TEXT DEFAULT 'open',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	`

	_, err = db.Exec(schema)
	return err
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func verifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateJWT(userID int, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func extractUserFromToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Missing authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid authorization header"})
			return
		}

		claims, err := extractUserFromToken(parts[1])
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid token"})
			return
		}

		// Store claims in context-like pattern using request header
		r.Header.Set("X-User-ID", fmt.Sprintf("%d", claims.UserID))
		r.Header.Set("X-User-Email", claims.Email)

		next.ServeHTTP(w, r)
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Handlers
func healthHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
		return
	}

	if req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Email and password required"})
		return
	}

	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error hashing password"})
		return
	}

	_, err = db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", req.Email, hashedPassword)
	if err != nil {
		respondJSON(w, http.StatusConflict, ErrorResponse{Error: "User already exists"})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"message": "User registered successfully"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
		return
	}

	if req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Email and password required"})
		return
	}

	var user User
	err := db.QueryRow("SELECT id, email, password FROM users WHERE email = ?", req.Email).
		Scan(&user.ID, &user.Email, &user.Password)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid credentials"})
		return
	}

	if !verifyPassword(user.Password, req.Password) {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid credentials"})
		return
	}

	token, err := generateJWT(user.ID, user.Email)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error generating token"})
		return
	}

	respondJSON(w, http.StatusOK, AuthResponse{Token: token, Email: user.Email})
}

func createTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	var req TicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
		return
	}

	if req.Title == "" {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Title required"})
		return
	}

	result, err := db.Exec(
		"INSERT INTO tickets (user_id, title, status) VALUES (?, ?, ?)",
		userID, req.Title, "open",
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error creating ticket"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error creating ticket"})
		return
	}

	ticket := Ticket{
		ID:        int(id),
		Title:     req.Title,
		Status:    "open",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	respondJSON(w, http.StatusCreated, ticket)
}

func listTicketsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	rows, err := db.Query(
		"SELECT id, user_id, title, status, created_at, updated_at FROM tickets WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error fetching tickets"})
		return
	}
	defer rows.Close()

	tickets := []Ticket{}
	for rows.Next() {
		var ticket Ticket
		if err := rows.Scan(&ticket.ID, &ticket.UserID, &ticket.Title, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt); err != nil {
			respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error scanning tickets"})
			return
		}
		tickets = append(tickets, ticket)
	}

	if tickets == nil {
		tickets = []Ticket{}
	}

	respondJSON(w, http.StatusOK, tickets)
}

func getTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Extract ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/tickets/")
	idStr = strings.Split(idStr, "/")[0]

	var ticket Ticket
	err := db.QueryRow(
		"SELECT id, user_id, title, status, created_at, updated_at FROM tickets WHERE id = ? AND user_id = ?",
		idStr, userID,
	).Scan(&ticket.ID, &ticket.UserID, &ticket.Title, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error fetching ticket"})
		return
	}

	respondJSON(w, http.StatusOK, ticket)
}

func updateTicketStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		respondJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Method not allowed"})
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		respondJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Extract ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/tickets/")
	idStr = strings.Split(idStr, "/")[0]

	var req StatusUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
		return
	}

	// Validate status
	validStatuses := map[string]bool{"open": true, "in_progress": true, "closed": true}
	if !validStatuses[req.Status] {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid status"})
		return
	}

	// Get current ticket
	var currentStatus string
	err := db.QueryRow("SELECT status FROM tickets WHERE id = ? AND user_id = ?", idStr, userID).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, ErrorResponse{Error: "Ticket not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error fetching ticket"})
		return
	}

	// Validate status transition
	if currentStatus == "closed" {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Cannot reopen a closed ticket"})
		return
	}

	// Valid transitions: open -> in_progress, in_progress -> closed
	validTransitions := map[string]map[string]bool{
		"open":        {"in_progress": true, "closed": true},
		"in_progress": {"closed": true},
		"closed":      {},
	}

	if !validTransitions[currentStatus][req.Status] {
		respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid status transition"})
		return
	}

	_, err = db.Exec(
		"UPDATE tickets SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?",
		req.Status, idStr, userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Error updating ticket"})
		return
	}

	var updatedTicket Ticket
	db.QueryRow(
		"SELECT id, user_id, title, status, created_at, updated_at FROM tickets WHERE id = ?",
		idStr,
	).Scan(&updatedTicket.ID, &updatedTicket.UserID, &updatedTicket.Title, &updatedTicket.Status, &updatedTicket.CreatedAt, &updatedTicket.UpdatedAt)

	respondJSON(w, http.StatusOK, updatedTicket)
}

func main() {
	if err := initDB(); err != nil {
		log.Fatal("Database initialization failed:", err)
	}
	defer db.Close()

	// Public routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/auth/register", registerHandler)
	http.HandleFunc("/auth/login", loginHandler)

	// Protected routes
	http.Handle("/tickets", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/status") {
			updateTicketStatusHandler(w, r)
		} else if r.Method == http.MethodPost {
			createTicketHandler(w, r)
		} else if r.Method == http.MethodGet {
			listTicketsHandler(w, r)
		}
	})))

	// Dynamic routes for ticket operations
	http.HandleFunc("/tickets/", func(w http.ResponseWriter, r *http.Request) {
		handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/status") {
				updateTicketStatusHandler(w, r)
			} else {
				getTicketHandler(w, r)
			}
		}))
		handler.ServeHTTP(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
