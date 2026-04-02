package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	pool      *pgxpool.Pool
	jwtSecret []byte
}

func NewHandler(pool *pgxpool.Pool, jwtSecret string) *Handler {
	return &Handler{pool: pool, jwtSecret: []byte(jwtSecret)}
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/refresh", h.Refresh)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		writeError(w, http.StatusBadRequest, "email, password and full_name are required")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID := uuid.New()
	now := time.Now().UTC()
	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, req.Email, string(hash), req.FullName, now, now)
	if err != nil {
		writeError(w, http.StatusConflict, "user with this email already exists")
		return
	}

	token, refreshToken, err := h.generateTokenPair(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusCreated, tokenResponse{Token: token, RefreshToken: refreshToken})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var userID uuid.UUID
	var passwordHash string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE email = $1`, req.Email).
		Scan(&userID, &passwordHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, refreshToken, err := h.generateTokenPair(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{Token: token, RefreshToken: refreshToken})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (any, error) {
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		writeError(w, http.StatusUnauthorized, "invalid token type")
		return
	}

	userIDStr, _ := claims["user_id"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token claims")
		return
	}

	newToken, newRefresh, err := h.generateTokenPair(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{Token: newToken, RefreshToken: newRefresh})
}

func (h *Handler) generateTokenPair(userID uuid.UUID) (string, string, error) {
	now := time.Now()

	accessClaims := jwt.MapClaims{
		"user_id": userID.String(),
		"type":    "access",
		"iat":     now.Unix(),
		"exp":     now.Add(15 * time.Minute).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString(h.jwtSecret)
	if err != nil {
		return "", "", err
	}

	refreshClaims := jwt.MapClaims{
		"user_id": userID.String(),
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     now.Add(7 * 24 * time.Hour).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString(h.jwtSecret)
	if err != nil {
		return "", "", err
	}

	return accessStr, refreshStr, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
