package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	roleKey   contextKey = "role"
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Authenticator struct {
	secret []byte
	issuer string
}

func NewAuthenticator(secret, issuer string) (*Authenticator, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, errors.New("JWT issuer must not be empty")
	}
	return &Authenticator{secret: []byte(secret), issuer: issuer}, nil
}

func (a *Authenticator) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			jsonAuthError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(
			parts[1], claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
				}
				return a.secret, nil
			},
			jwt.WithIssuer(a.issuer),
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)
		if err != nil || !token.Valid {
			jsonAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		ctx := WithIdentity(r.Context(), strconv.FormatInt(claims.UserID, 10), claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithIdentity(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, roleKey, role)
}

func UserID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(userIDKey).(string)
	return value, ok && value != ""
}

func Role(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(roleKey).(string)
	return value, ok && value != ""
}

func jsonAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
