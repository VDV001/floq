package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	secret := []byte(jwtSecret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
				return secret, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			tokenType, _ := claims["type"].(string)
			if tokenType != "access" {
				http.Error(w, `{"error":"invalid token type"}`, http.StatusUnauthorized)
				return
			}

			userIDStr, _ := claims["user_id"].(string)
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "user_id", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
