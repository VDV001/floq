package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	repo      UserRepository
	jwtSecret []byte
}

func NewHandler(repo UserRepository, jwtSecret string) *Handler {
	return &Handler{repo: repo, jwtSecret: []byte(jwtSecret)}
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
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email, password and full_name are required")
		return
	}
	if len(req.Password) < 6 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID := uuid.New()
	now := time.Now().UTC()
	if err = h.repo.CreateUser(r.Context(), userID, req.Email, string(hash), req.FullName, now); err != nil {
		httputil.WriteError(w, http.StatusConflict, "user with this email already exists")
		return
	}

	token, refreshToken, err := h.generateTokenPair(userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, tokenResponse{Token: token, RefreshToken: refreshToken})
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
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	userID, passwordHash, err := h.repo.FindUserByEmail(r.Context(), req.Email)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, refreshToken, err := h.generateTokenPair(userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, tokenResponse{Token: token, RefreshToken: refreshToken})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		httputil.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (any, error) {
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid token type")
		return
	}

	userIDStr, _ := claims["user_id"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid token claims")
		return
	}

	newToken, newRefresh, err := h.generateTokenPair(userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, tokenResponse{Token: newToken, RefreshToken: newRefresh})
}

func (h *Handler) generateTokenPair(userID uuid.UUID) (string, string, error) {
	now := time.Now()

	accessClaims := jwt.MapClaims{
		"user_id": userID.String(),
		"type":    "access",
		"iat":     now.Unix(),
		"exp":     now.Add(24 * time.Hour).Unix(),
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

